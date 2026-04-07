package dockerfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/sourceresolver"
	"github.com/moby/buildkit/frontend/dockerui"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/frontend/subrequests/convertllb"
	"github.com/moby/buildkit/solver/pb"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/encoding/protojson"
)

const PATH = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

func Build(ctx context.Context, c client.Client) (*client.Result, error) {
	opts := c.BuildOpts().Opts
	if _, ok := opts["filename"]; !ok {
		opts["filename"] = "dockerfile.nix"
	}

	bc, err := dockerui.NewClient(c)
	if err != nil {
		return nil, err
	}

	if bc.Target == "" {
		bc.Target = "default"
	}

	var frontendImg llb.State
	if cc, ok := c.(cf); ok {
		baseImg, err := cc.CurrentFrontend()
		if err != nil {
			return nil, fmt.Errorf("unable to determine current frontend: %w", err)
		}

		frontendImg = baseImg.AddEnv("PATH", PATH)
	}

	src, err := bc.ReadEntrypoint(ctx, "dockerfile-nix")
	if err != nil {
		return nil, err
	}

	inputs, err := resolveInputs(ctx, c, src, frontendImg)
	if err != nil {
		return nil, err
	}

	runArgs := []string{
		"nix-solve",
		"-f", path.Join("/src", path.Base(src.Filename)),
		"-t", bc.Target,
		"-o", "/result/dockerfile.json",
		"-c", "/inputs/config.json",
	}

	var bp ocispecs.Platform
	if len(bc.BuildPlatforms) > 0 {
		bp = bc.BuildPlatforms[0]
	} else {
		bp = platforms.DefaultSpec()
	}

	toSolveRequest := func(ctx context.Context, platform *ocispecs.Platform, idx int) (client.SolveRequest, *dockerspec.DockerOCIImage, error) {
		var tp ocispecs.Platform
		if platform != nil {
			tp = *platform
		} else {
			tp = bp
		}

		configuration, err := json.Marshal(getConfiguration(bc, bp, tp))
		if err != nil {
			return client.SolveRequest{}, nil, err
		}

		runOpts := []llb.RunOption{
			llb.WithCustomNamef("[dockerfile] resolving %s", src.Filename),
			llb.Args(runArgs),
			llb.AddMount("/src", *src.State),
			llb.AddMount("/inputs", llb.Scratch().
				File(
					llb.Mkfile("config.json", 0o444, configuration),
				),
				llb.Readonly,
			),
			llb.AddMount("/nix/store", llb.Scratch(), llb.AsPersistentCacheDir("nix-frontend-store", llb.CacheMountShared)),
		}

		for k, st := range inputs {
			dest := filepath.Join("/nix/var/nix", k)
			runOpts = append(runOpts, llb.AddMount(dest, st))
		}

		st := frontendImg.
			Run(runOpts...).
			AddMount("/result", llb.Scratch())

		def, err := st.Marshal(ctx)
		if err != nil {
			return client.SolveRequest{}, nil, err
		}

		req := client.SolveRequest{
			Definition: def.ToPB(),
		}
		res, err := c.Solve(ctx, req)
		if err != nil {
			return client.SolveRequest{}, nil, err
		}

		ref, err := res.SingleRef()
		if err != nil {
			return client.SolveRequest{}, nil, err
		}

		in, err := ref.ReadFile(ctx, client.ReadRequest{
			Filename: "dockerfile.json",
		})
		if err != nil {
			return client.SolveRequest{}, nil, err
		}

		outDef := &pb.Definition{}
		if err := protojson.Unmarshal(in, outDef); err != nil {
			return client.SolveRequest{}, nil, err
		}

		gr, err := newGraph(outDef)
		if err != nil {
			return client.SolveRequest{}, nil, err
		}

		if err := resolveContexts(ctx, bc, gr, tp); err != nil {
			return client.SolveRequest{}, nil, err
		}

		img, err := resolveImages(ctx, c, gr, tp)
		if err != nil {
			return client.SolveRequest{}, nil, err
		}

		outDef, err = gr.ToDef()
		if err != nil {
			return client.SolveRequest{}, nil, err
		}

		solveReq := client.SolveRequest{
			Definition: outDef,
		}
		return solveReq, img, nil
	}

	if res, ok, err := bc.HandleSubrequest(ctx, dockerui.RequestHandler{
		ConvertLLB: func(ctx context.Context) (*convertllb.Result, error) {
			req, _, err := toSolveRequest(ctx, &bp, 0)
			if err != nil {
				return nil, err
			}
			return convertLLBResult(req)
		},
	}); err != nil {
		return nil, err
	} else if ok {
		return res, nil
	}

	res, err := bc.Build(ctx, func(ctx context.Context, platform *ocispecs.Platform, idx int) (*dockerui.BuildResult, error) {
		req, img, err := toSolveRequest(ctx, platform, idx)
		if err != nil {
			return nil, err
		}

		res, err := c.Solve(ctx, req)
		if err != nil {
			return nil, err
		}

		r, err := res.SingleRef()
		return &dockerui.BuildResult{
			Reference: r,
			Image:     img,
			BaseImage: img,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return res.Finalize()
}

type Image struct {
	Ref    string
	Digest digest.Digest
	dockerspec.DockerOCIImage
}

func resolveImages(ctx context.Context, c client.Client, gr *graph, tp ocispecs.Platform) (*dockerspec.DockerOCIImage, error) {
	imgs, err := resolveImageConfigs(ctx, c, gr, tp)
	if err != nil {
		return nil, err
	}

	for dgst, v := range gr.All() {
		var img *Image
		switch o := v.Op.Op.(type) {
		case *pb.Op_Source:
			if !strings.HasPrefix(o.Source.Identifier, "docker-image://") {
				continue
			}
			ref := strings.TrimPrefix(o.Source.Identifier, "docker-image://")
			img = imgs[ref]
			o.Source.Identifier = "docker-image://" + img.Ref
		case *pb.Op_Exec:
			for _, m := range o.Exec.Mounts {
				if m.Dest == "/" && m.Input >= 0 {
					inp := v.Op.Inputs[m.Input]
					if img = imgs[inp.Digest]; img != nil {
						config := img.Config
						if o.Exec.Meta.Cwd == "" {
							o.Exec.Meta.Cwd = config.WorkingDir
						}
						o.Exec.Meta.Env = append(config.Env, o.Exec.Meta.Env...)
						if o.Exec.Meta.User == "" {
							o.Exec.Meta.User = config.User
						}
						break
					}
				}
			}
		default:
			if len(v.Op.Inputs) > 0 {
				inp := v.Op.Inputs[0]
				img = imgs[inp.Digest]
			}
		}

		if img != nil {
			// Any attributes to update?
			if dt, ok := v.Meta.Description["oci.image.config"]; ok {
				var config dockerspec.DockerOCIImageConfig
				if err := json.Unmarshal([]byte(dt), &config); err != nil {
					return nil, err
				}

				// Merge in attributes from the config.
				cloneImg := *img
				mergeImageConfig(&cloneImg.Config, &config)
				img = &cloneImg
			}
			imgs[string(dgst)] = img
		}
	}

	head, _ := gr.Head()
	if img := imgs[string(head)]; img != nil {
		return &img.DockerOCIImage, nil
	}
	return nil, nil
}

func mergeImageConfig(into, from *dockerspec.DockerOCIImageConfig) {
	if len(from.Cmd) > 0 {
		into.Cmd = from.Cmd
	}
	if len(from.Entrypoint) > 0 {
		into.Entrypoint = from.Entrypoint
	}
	if from.WorkingDir != "" {
		into.WorkingDir = from.WorkingDir
	}
	if from.User != "" {
		into.User = from.User
	}
}

func resolveImageConfigs(ctx context.Context, c client.Client, gr *graph, tp ocispecs.Platform) (map[string]*Image, error) {
	m := sync.Map{}
	seen := make(map[string]struct{})

	eg, ctx := errgroup.WithContext(ctx)
	defer eg.Wait()

	if err := gr.Walk(func(op *pb.Op) error {
		platform := tp
		if op.Platform != nil {
			platform = op.Platform.Spec()
		}

		switch op := op.Op.(type) {
		case *pb.Op_Source:
			if !strings.HasPrefix(op.Source.Identifier, "docker-image://") {
				return nil
			}

			refName := strings.TrimPrefix(op.Source.Identifier, "docker-image://")
			named, err := reference.ParseNormalizedNamed(refName)
			if err != nil {
				return err
			}
			refName = reference.TagNameOnly(named).String()
			op.Source.Identifier = "docker-image://" + refName
			if _, ok := seen[refName]; ok {
				return nil
			}
			seen[refName] = struct{}{}

			eg.Go(func() error {
				ref, dgst, dt, err := c.ResolveImageConfig(ctx, refName, sourceresolver.Opt{
					ImageOpt: &sourceresolver.ResolveImageOpt{
						Platform: &platform,
					},
				})
				if err != nil {
					return err
				}

				var img dockerspec.DockerOCIImage
				if err := json.Unmarshal(dt, &img); err != nil {
					return err
				}

				m.Store(refName, &Image{
					Ref:            ref,
					Digest:         dgst,
					DockerOCIImage: img,
				})
				return nil
			})
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	out := make(map[string]*Image)
	for k, v := range m.Range {
		out[k.(string)] = v.(*Image)
	}
	return out, nil
}

func resolveContexts(ctx context.Context, bc *dockerui.Client, gr *graph, tp ocispecs.Platform) error {
	for _, v := range gr.All() {
		switch o := v.Op.Op.(type) {
		case *pb.Op_Source:
			u, err := url.Parse(o.Source.Identifier)
			if err != nil {
				continue
			}

			if u.Scheme != "context" {
				continue
			}
			name := u.Host

			var st *llb.State
			if name == "context" {
				st, err = bc.MainContext(ctx)
				if err != nil {
					return err
				}
			} else {
				nc, err := bc.NamedContext(name, dockerui.ContextOpt{
					Platform: &tp,
				})
				if err != nil {
					return err
				}

				st, _, err = nc.Load(ctx)
				if err != nil {
					return err
				}
			}

			constraints := &llb.Constraints{}
			vtx := st.Output().Vertex(ctx, constraints)
			if len(vtx.Inputs()) != 0 {
				return errors.New("expected context to have no inputs")
			}

			// This seems to be the only way to get the op out of the interface
			// is to marshal it and then unmarshal it back out. Can probably
			// improve this so we just store the marshaled output and use it directly
			// since this is going to just get marshaled again later anyway.
			// But it messes too much with the flow control to bother with that right now.
			_, dt, _, _, err := vtx.Marshal(ctx, constraints)
			if err != nil {
				return err
			}

			var newOp pb.Op
			if err := newOp.Unmarshal(dt); err != nil {
				return err
			}

			// Inherit the attributes from the original source.
			if newOp, ok := newOp.Op.(*pb.Op_Source); ok {
				if u, err := url.Parse(newOp.Source.Identifier); err == nil {
					prefix := u.Scheme + "."
					for k, v := range o.Source.Attrs {
						if strings.HasPrefix(k, prefix) {
							if newOp.Source.Attrs == nil {
								newOp.Source.Attrs = make(map[string]string)
							}
							newOp.Source.Attrs[k] = v
						}
					}
				}
			}
			v.Op.Op = newOp.Op
		}
	}
	return nil
}

func resolveInputs(ctx context.Context, c client.Client, src *dockerui.Source, frontendImg llb.State) (map[string]llb.State, error) {
	runArgs := []string{
		"nix-resolve-inputs",
		"-f", path.Join("/src", path.Base(src.Filename)),
		"-o", "/result/inputs.json",
	}

	runOpts := []llb.RunOption{
		llb.WithCustomNamef("[dockerfile] resolving inputs for %s", src.Filename),
		llb.Args(runArgs),
		llb.AddMount("/src", *src.State),
		llb.AddMount("/nix/store", llb.Scratch(), llb.AsPersistentCacheDir("nix-frontend-store", llb.CacheMountShared)),
	}

	st := frontendImg.
		Run(runOpts...).
		AddMount("/result", llb.Scratch())

	def, err := st.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	req := client.SolveRequest{
		Definition: def.ToPB(),
	}
	res, err := c.Solve(ctx, req)
	if err != nil {
		return nil, err
	}

	ref, err := res.SingleRef()
	if err != nil {
		return nil, err
	}

	in, err := ref.ReadFile(ctx, client.ReadRequest{
		Filename: "inputs.json",
	})
	if err != nil {
		return nil, err
	}

	rawInputs := map[string]json.RawMessage{}
	if err := json.Unmarshal(in, &rawInputs); err != nil {
		return nil, err
	}

	inputMap := make(map[string]llb.State, len(rawInputs))
	for k, b := range rawInputs {
		def := &pb.Definition{}
		if err := protojson.Unmarshal(b, def); err != nil {
			return nil, err
		}

		op, err := llb.NewDefinitionOp(def)
		if err != nil {
			return nil, err
		}
		inputMap[k] = llb.NewState(op)
	}
	return inputMap, nil
}

type Config struct {
	BuildArgs map[string]string   `json:"build-arg"`
	Contexts  map[string]struct{} `json:"contexts"`
	Context   string              `json:"context"`
}

const contextPrefix = "context:"

func getConfiguration(bc *dockerui.Client, bp, tp ocispecs.Platform) *Config {
	config := &Config{}
	config.BuildArgs = getBuildArgs(bc, bp, tp)

	config.Contexts = make(map[string]struct{})
	for key := range bc.BuildOpts().Opts {
		if name, found := strings.CutPrefix(key, contextPrefix); found {
			config.Contexts[name] = struct{}{}
		}
	}
	config.Context = "context"
	return config
}

func getBuildArgs(bc *dockerui.Client, bp, tp ocispecs.Platform) map[string]string {
	s := [...][2]string{
		{"buildOs", bp.OS},
		{"buildOsVersion", bp.OSVersion},
		{"buildArch", bp.Architecture},
		{"buildVariant", bp.Variant},
		{"targetOs", tp.OS},
		{"targetOsVersion", tp.OSVersion},
		{"targetArch", tp.Architecture},
		{"targetVariant", tp.Variant},
		{"targetStage", bc.Target},
	}

	args := make(map[string]string, len(bc.BuildArgs)+len(s))
	for _, kv := range s {
		k, v := kv[0], kv[1]
		args[k] = v
	}

	for k, v := range bc.BuildArgs {
		k = toLowerCamelCase(k)
		args[k] = v
	}
	return args
}

var lowerCamelCase = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`_(.)`)
})

func toLowerCamelCase(s string) string {
	return lowerCamelCase().ReplaceAllStringFunc(strings.ToLower(s), func(s string) string {
		return strings.ToUpper(s[1:])
	})
}

type cf interface {
	CurrentFrontend() (*llb.State, error)
}

func convertLLBResult(req client.SolveRequest) (*convertllb.Result, error) {
	res := &convertllb.Result{
		Def:      make(map[digest.Digest]*pb.Op),
		Metadata: make(map[digest.Digest]llb.OpMetadata),
		Source:   req.Definition.Source,
	}
	for _, def := range req.Definition.Def {
		var op pb.Op
		if err := op.Unmarshal(def); err != nil {
			return nil, err
		}

		dgst := digest.FromBytes(def)
		res.Def[dgst] = &op
	}

	for dgst, mpb := range req.Definition.Metadata {
		var opMeta llb.OpMetadata
		opMeta.FromPB(mpb)
		res.Metadata[digest.Digest(dgst)] = opMeta
	}
	return res, nil
}
