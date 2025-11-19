package dockerfile

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/distribution/reference"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/sourceresolver"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/solver/pb"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/opencontainers/go-digest"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/encoding/protojson"
)

func Build(ctx context.Context, c client.Client) (*client.Result, error) {
	target := c.BuildOpts().Opts["target"]
	if target == "" {
		target = "default"
	}

	runArgs := []string{
		"nix-solve",
		"-f", "/src/dockerfile.nix",
		"-t", target,
		"-o", "/result/dockerfile.json",
	}

	buildArgs := getBuildArgs(c)
	if len(buildArgs) > 0 {
		runArgs = append(runArgs, "-a", "/inputs/args.json")
	}

	runOpts := []llb.RunOption{
		llb.WithCustomNamef("[dockerfile] resolving %s", "dockerfile.nix"),
		llb.Args(runArgs),
		llb.AddMount("/src", llb.Local("dockerfile", llb.FollowPaths([]string{"dockerfile.nix"}))),
	}
	if len(buildArgs) > 0 {
		args, err := json.Marshal(buildArgs)
		if err != nil {
			return nil, err
		}

		inputs := llb.Scratch().
			File(
				llb.Mkfile("args.json", 0444, args),
			)
		runOpts = append(runOpts, llb.AddMount("/inputs", inputs, llb.Readonly))
	}

	inputs, err := resolveInputs(ctx, c)
	if err != nil {
		return nil, err
	}

	for k, st := range inputs {
		dest := filepath.Join("/nix/var/nix/profiles/per-user/root/channels", k)
		runOpts = append(runOpts, llb.AddMount(dest, st))
	}

	imageRef := fmt.Sprintf("%s:%s-nix", Repository, Version)
	st := llb.Image(imageRef, llb.WithMetaResolver(c)).
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
		Filename: "dockerfile.json",
	})
	if err != nil {
		return nil, err
	}

	outDef := &pb.Definition{}
	if err := protojson.Unmarshal(in, outDef); err != nil {
		return nil, err
	}

	gr, err := newGraph(outDef)
	if err != nil {
		return nil, err
	}

	img, err := resolveImages(ctx, c, gr)
	if err != nil {
		return nil, err
	}

	outDef, err = gr.ToDef()
	if err != nil {
		return nil, err
	}

	res, err = c.Solve(ctx, client.SolveRequest{
		Definition: outDef,
	})
	if err != nil {
		return nil, err
	}

	if img != nil {
		dt, err := json.Marshal(img)
		if err != nil {
			return nil, err
		}
		res.AddMeta(exptypes.ExporterImageConfigKey, dt)
	}
	return res, nil
}

type Image struct {
	Ref    string
	Digest digest.Digest
	dockerspec.DockerOCIImage
}

func resolveImages(ctx context.Context, c client.Client, gr *graph) (*dockerspec.DockerOCIImage, error) {
	imgs, err := resolveImageConfigs(ctx, c, gr)
	if err != nil {
		return nil, err
	}

	for dgst, op := range gr.All() {
		switch o := op.Op.(type) {
		case *pb.Op_Source:
			if !strings.HasPrefix(o.Source.Identifier, "docker-image://") {
				continue
			}

			ref := strings.TrimPrefix(o.Source.Identifier, "docker-image://")
			config := imgs[ref]
			o.Source.Identifier = "docker-image://" + config.Ref
			imgs[string(dgst)] = config
		case *pb.Op_Exec:
			for _, m := range o.Exec.Mounts {
				if m.Dest == "/" && m.Input >= 0 {
					inp := op.Inputs[m.Input]
					if img := imgs[inp.Digest]; img != nil {
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
			if len(op.Inputs) > 0 {
				inp := op.Inputs[0]
				imgs[string(dgst)] = imgs[inp.Digest]
			}
		}
	}

	head, _ := gr.Head()
	if img := imgs[string(head)]; img != nil {
		return &img.DockerOCIImage, nil
	}
	return nil, nil
}

func resolveImageConfigs(ctx context.Context, c client.Client, gr *graph) (map[string]*Image, error) {
	m := sync.Map{}
	seen := make(map[string]struct{})

	eg, ctx := errgroup.WithContext(ctx)
	defer eg.Wait()

	if err := gr.Walk(func(op *pb.Op) error {
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
				ref, dgst, dt, err := c.ResolveImageConfig(ctx, refName, sourceresolver.Opt{})
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

func resolveInputs(ctx context.Context, c client.Client) (map[string]llb.State, error) {
	runArgs := []string{
		"nix-resolve-inputs",
		"-f", "/src/dockerfile.nix",
		"-o", "/result/inputs.json",
	}

	runOpts := []llb.RunOption{
		llb.WithCustomNamef("[dockerfile] resolving inputs for %s", "dockerfile.nix"),
		llb.Args(runArgs),
		llb.AddMount("/src", llb.Local("dockerfile", llb.FollowPaths([]string{"dockerfile.nix"}))),
	}

	imageRef := fmt.Sprintf("%s:%s-nix", Repository, Version)
	st := llb.Image(imageRef, llb.WithMetaResolver(c)).
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

func getBuildArgs(c client.Client) map[string]string {
	args := make(map[string]string)
	for k, v := range c.BuildOpts().Opts {
		k, found := strings.CutPrefix(k, "build-arg:")
		if !found {
			continue
		}

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
