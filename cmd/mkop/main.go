package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jsternberg/nix-frontend/dockerfile"
	"github.com/moby/buildkit/solver/pb"
	"github.com/urfave/cli/v2"
)

func mkop(d string, infile string) error {
	spec, err := dockerfile.ReadOpSpec(infile)
	if err != nil {
		return err
	}

	var op *pb.Op
	switch {
	case spec.Source != nil:
		op, err = convertSourceOp(spec.Source)
		if err != nil {
			return err
		}
	case spec.Exec != nil:
		op, err = convertExecOp(spec.Exec)
		if err != nil {
			return err
		}
	case spec.File != nil:
		op, err = convertFileOp(spec.File)
		if err != nil {
			return err
		}
	case spec.Merge != nil:
		op, err = convertMergeOp(spec.Merge)
		if err != nil {
			return err
		}
	}

	if op == nil {
		return errors.New("invalid operation spec")
	}

	v := &dockerfile.Vertex{
		Op: op,
	}
	if spec.Meta != nil {
		if v.Meta == nil {
			v.Meta = &dockerfile.Metadata{}
		}
		v.Meta.Description = spec.Meta.Description
	}

	if err := WriteJSON(v, d, "vertex.json"); err != nil {
		return err
	}

	index := map[string]string{
		"/": filepath.Join(d, "vertex.json"),
	}

	switch op := op.Op.(type) {
	case *pb.Op_Exec:
		for _, mount := range op.Exec.Mounts {
			if mount.Dest == "/" || mount.Output < 0 {
				continue
			}

			fpath := filepath.Join(d, strconv.FormatInt(mount.Output, 10))
			if err := os.Mkdir(fpath, 0755); err != nil && !os.IsExist(err) {
				return err
			}

			v := &dockerfile.Vertex{
				Op: &pb.Op{
					Inputs: []*pb.Input{
						{
							Digest: filepath.Join(d, "vertex.json"),
							Index:  mount.Output,
						},
					},
				},
			}
			if err := WriteJSON(v, fpath, "vertex.json"); err != nil {
				return err
			}
			index[mount.Dest] = filepath.Join(fpath, "vertex.json")
		}
	}

	if err := WriteJSON(index, d, "index.json"); err != nil {
		return err
	}
	return nil
}

func WriteJSON(v any, paths ...string) error {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return err
	}

	fpath := filepath.Join(paths...)
	return os.WriteFile(fpath, data, 0644)
}

func main() {
	app := cli.NewApp()
	app.Usage = "mkop"
	app.Action = func(c *cli.Context) error {
		if c.NArg() < 1 {
			return errors.New("expected at least one argument")
		} else if c.NArg() > 2 {
			return errors.New("too many arguments")
		}

		args := c.Args()
		infile := os.ExpandEnv(args.Get(0))

		outdir := "."
		if c.NArg() == 2 {
			outdir = os.ExpandEnv(args.Get(1))
			if err := os.Mkdir(outdir, 0755); err != nil {
				return err
			}
		}
		return mkop(outdir, infile)
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "fatal error: %+v\n", err)
		os.Exit(1)
	}
}
