package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jsternberg/nix-frontend/dockerfile"
	"github.com/moby/buildkit/solver/pb"
	"github.com/opencontainers/go-digest"
	"github.com/urfave/cli/v2"
	"google.golang.org/protobuf/encoding/protojson"
)

type Vertex = dockerfile.Vertex

func load(input string) (map[string]*Vertex, []string, error) {
	unvisited := make(map[string]struct{})
	unvisited[input] = struct{}{}
	order := []string{input}

	loaded := make(map[string]*Vertex)
	for len(unvisited) > 0 {
		path, _ := pop(unvisited)

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}

		v := &Vertex{}
		if err := json.Unmarshal(data, v); err != nil {
			return nil, nil, err
		}

		for _, input := range v.Op.Inputs {
			inputPath := input.Digest

			// Store the order we are loading files.
			// We add this even if the file has been loaded before because
			// we want to capture the full reverse load order.
			order = append(order, inputPath)

			// Check if we have already loaded the file.
			if _, ok := loaded[inputPath]; ok {
				// Already visited.
				continue
			}
			unvisited[inputPath] = struct{}{}
		}
		loaded[path] = v
	}

	reverse(order)

	if len(order) > 0 {
		loaded["result"] = &Vertex{
			Op: &pb.Op{
				Inputs: []*pb.Input{
					{Digest: order[len(order)-1]},
				},
			},
		}
		order = append(order, "result")
	}
	return loaded, order, nil
}

func convert(specs map[string]*Vertex, order []string) (*pb.Definition, error) {
	def := &pb.Definition{
		Metadata: make(map[string]*pb.OpMetadata),
	}
	outputs := make(map[string]*pb.Input)

	for _, path := range order {
		if _, ok := outputs[path]; ok {
			continue
		}

		v := specs[path]
		if v == nil {
			return nil, errors.New("cycle detected")
		}

		for i, inp := range v.Op.Inputs {
			v.Op.Inputs[i] = outputs[inp.Digest].CloneVT()
			v.Op.Inputs[i].Index = inp.Index
		}

		b, err := v.Op.Marshal()
		if err != nil {
			return nil, err
		}
		def.Def = append(def.Def, b)

		out := &pb.Input{
			Digest: string(digest.FromBytes(b)),
		}
		if v.Meta == nil {
			v.Meta = &dockerfile.Metadata{}
		}

		if v.Meta.Description == nil {
			v.Meta.Description = map[string]string{}
		}

		src, _ := protojson.Marshal(v.Op)
		v.Meta.Description["llb.source"] = string(src)
		if v.Meta != nil {
			def.Metadata[out.Digest] = &pb.OpMetadata{
				Description: v.Meta.Description,
			}
		}
		outputs[path] = out
	}
	return def, nil
}

func normalizeAndOptimize(specs map[string]*Vertex, order []string) {
	injectInferredMergeOp(specs, order)
	unrollTrivialMerges(specs, order)
}

func injectInferredMergeOp(specs map[string]*Vertex, order []string) {
	// We only need the inferred merge op if this op is used as a dependency
	// of another operation. So we iterate backwards through the order and insert
	// the merge op for inputs. This means the last vertex should never have
	// a merge op.
	for i := len(order) - 1; i >= 0; i-- {
		p := order[i]

		v := specs[p]
		for _, inp := range v.Op.Inputs {
			vinp := specs[inp.Digest]
			if vinp.Op.Op == nil {
				inputs := make([]*pb.MergeInput, len(vinp.Op.Inputs))
				for i := range inputs {
					inputs[i] = &pb.MergeInput{Input: int64(i)}
				}
				vinp.Op.Op = &pb.Op_Merge{
					Merge: &pb.MergeOp{
						Inputs: inputs,
					},
				}
			}
		}
	}
}

func unrollTrivialMerges(specs map[string]*Vertex, order []string) {
	for _, p := range order {
		v := specs[p]
		for _, inp := range v.Op.Inputs {
			for {
				vinp := specs[inp.Digest]
				if op, ok := vinp.Op.Op.(*pb.Op_Merge); ok {
					if len(op.Merge.Inputs) == 1 && op.Merge.Inputs[0].Input == 0 {
						inp.Digest = vinp.Op.Inputs[0].Digest
						inp.Index = vinp.Op.Inputs[0].Index
						continue
					}
				}
				break
			}
		}
	}
}

func marshal(out io.Writer, infile string) error {
	specs, order, err := load(infile)
	if err != nil {
		return err
	}
	normalizeAndOptimize(specs, order)

	def, err := convert(specs, order)
	if err != nil {
		return err
	}

	src, err := marshalJSON(def)
	if err != nil {
		return err
	}

	if _, err := out.Write(src); err != nil {
		return err
	}
	return nil
}

func main() {
	app := cli.NewApp()
	app.Usage = "marshal"
	app.Action = func(c *cli.Context) error {
		if c.NArg() < 1 {
			return errors.New("expected at least one argument")
		} else if c.NArg() > 2 {
			return errors.New("too many arguments")
		}

		args := c.Args()
		infile := filepath.Join(os.ExpandEnv(args.Get(0)), "vertex.json")

		out := os.Stdout
		if c.NArg() == 2 {
			outfile := os.ExpandEnv(args.Get(1))
			f, err := os.Create(outfile)
			if err != nil {
				return err
			}
			defer f.Close()

			out = f
		}
		return marshal(out, infile)
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "fatal error: %+v\n", err)
		os.Exit(1)
	}
}

func pop[K comparable, V any](set map[K]V) (k K, v V) {
	for k, v = range set {
		delete(set, k)
		return k, v
	}
	return k, v
}

func reverse[T any](arr []T) {
	for i, j := 0, len(arr)-1; i < j; i, j = i+1, j-1 {
		arr[i], arr[j] = arr[j], arr[i]
	}
}

func marshalJSON(def *pb.Definition) ([]byte, error) {
	src, err := protojson.Marshal(def)
	if err != nil {
		return nil, err
	}

	var data bytes.Buffer
	if err := json.Indent(&data, src, "", "\t"); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}
