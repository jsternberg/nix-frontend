package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/jsternberg/nix-frontend/dockerfile"
	"github.com/moby/buildkit/solver/pb"
)

func convertSourceOp(in *dockerfile.SourceOp) (*pb.Op, error) {
	var attrs map[string]string
	if len(in.Attributes) > 0 {
		u, err := url.Parse(in.Identifier)
		if err != nil {
			return nil, err
		}

		attrs = make(map[string]string)
		for k, v := range in.Attributes {
			attrs[fmt.Sprintf("%s.%s", u.Scheme, k)] = v
		}
	}

	return &pb.Op{
		Op: &pb.Op_Source{
			Source: &pb.SourceOp{
				Identifier: in.Identifier,
				Attrs:      attrs,
			},
		},
	}, nil
}

func convertExecOp(in *dockerfile.ExecOp) (*pb.Op, error) {
	exec := &pb.ExecOp{
		Meta: &pb.Meta{
			Args: in.Command,
			Cwd:  in.Workdir,
			Env:  in.Env,
		},
	}

	out := &pb.Op{
		Op: &pb.Op_Exec{
			Exec: exec,
		},
	}

	paths := make([]string, 0, len(in.Mounts))
	for path := range in.Mounts {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	numOutputs := 0
	for _, path := range paths {
		spec := in.Mounts[path]

		mountType := pb.MountType_BIND
		switch spec.Type {
		case "tmpfs":
			mountType = pb.MountType_TMPFS
		case "cache":
			mountType = pb.MountType_CACHE
		}

		var sel string
		input := pb.Empty
		if spec.Input != "" {
			var err error
			input, sel, err = resolveInput(out, spec.Input)
			if err != nil {
				return nil, err
			}
		}

		output := -1
		if path == "/" {
			output = 0
		}

		mount := &pb.Mount{
			MountType: mountType,
			Input:     int64(input),
			Selector:  sel,
			Dest:      path,
			Output:    int64(output),
			Readonly:  spec.Readonly,
		}

		switch mount.MountType {
		case pb.MountType_TMPFS:
			mount.TmpfsOpt = &pb.TmpfsOpt{}
		case pb.MountType_CACHE:
			mount.CacheOpt = &pb.CacheOpt{}
		default:
			if !mount.Readonly && mount.Output < 0 {
				// Assign a mount output.
				numOutputs++
				mount.Output = int64(numOutputs)
			}
		}
		exec.Mounts = append(exec.Mounts, mount)
	}
	return out, nil
}

func convertFileOp(in *dockerfile.FileOp) (*pb.Op, error) {
	op := &pb.Op{}

	inp := MergeInput{
		Index: -1,
	}
	if in.Target != "" {
		var err error
		inp.Index, inp.Path, err = resolveInput(op, in.Target)
		if err != nil {
			return nil, err
		}
	}

	paths := make([]string, 0, len(in.Locations))
	for p := range in.Locations {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	sources := make(map[string]*MergeInput)
	for _, p := range paths {
		if entry := in.Locations[p]; entry.Source != "" {
			index, path, err := resolveInput(op, entry.Source)
			if err != nil {
				return nil, err
			}
			sources[entry.Source] = &MergeInput{
				Index: index,
				Path:  path,
			}
		}
	}

	file := &pb.FileOp{}

	if inp.Path != "" {
		action := &pb.FileAction{
			Input:          -1,
			SecondaryInput: int64(inp.Index),
			Output:         -1,
			Action: &pb.FileAction_Copy{
				Copy: &pb.FileActionCopy{
					Src:  inp.Path,
					Dest: "/",
					Mode: -1,
				},
			},
		}
		file.Actions = append(file.Actions, action)
		inp.Index, inp.Path = pb.InputIndex(len(op.Inputs)+len(file.Actions)-1), ""
	}

	for _, p := range paths {
		action := &pb.FileAction{
			Input:          int64(inp.Index),
			SecondaryInput: -1,
			Output:         -1,
		}

		switch entry := in.Locations[p]; {
		case entry.Source != "":
			inp := sources[entry.Source]
			action.SecondaryInput = int64(inp.Index)
			action.Action = &pb.FileAction_Copy{
				Copy: &pb.FileActionCopy{
					Src:  inp.Path,
					Dest: p,
				},
			}
		case entry.Text != "":
			action.Action = &pb.FileAction_Mkfile{
				Mkfile: &pb.FileActionMkFile{
					Path: p,
					Data: []byte(entry.Text),
					Mode: 0644,
				},
			}
		}

		if action.Action != nil {
			file.Actions = append(file.Actions, action)
			inp.Index = pb.InputIndex(len(op.Inputs) + len(file.Actions) - 1)
		}
	}

	if len(file.Actions) > 0 {
		file.Actions[len(file.Actions)-1].Output = 0
	}

	op.Op = &pb.Op_File{File: file}
	return op, nil
}

type MergeInput struct {
	Index pb.InputIndex
	Path  string
}

func convertMergeOp(in *dockerfile.MergeOp) (*pb.Op, error) {
	op := &pb.Op{}

	var (
		target = MergeInput{
			Index: -1,
		}
		inputs []*MergeInput
	)
	if in.Target != "" {
		var err error
		target.Index, target.Path, err = resolveInput(op, in.Target)
		if err != nil {
			return nil, err
		}
	}

	if target.Path == "" {
		target.Path = "/"
	}

	for _, input := range in.Inputs {
		index, path, err := resolveInput(op, input)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, &MergeInput{
			Index: index,
			Path:  path,
		})
	}

	if len(inputs) == 0 {
		return op, nil
	} else if canMerge(target, inputs) {
		return mkMerge(op, target.Index, inputs)
	}

	file := &pb.FileOp{}
	offset := len(op.Inputs)
	for i, input := range inputs {
		src := input.Path
		if src == "" {
			src = "/"
		}

		action := &pb.FileAction{
			Input:          int64(target.Index),
			SecondaryInput: int64(input.Index),
			Output:         -1,
			Action: &pb.FileAction_Copy{
				Copy: &pb.FileActionCopy{
					Src:  src,
					Dest: target.Path,
					Mode: -1,
				},
			},
		}
		if i == len(inputs)-1 {
			action.Output = 0
		}
		target.Index = pb.InputIndex(len(file.Actions) + offset)

		file.Actions = append(file.Actions, action)
	}

	op.Op = &pb.Op_File{File: file}
	return op, nil
}

func canMerge(target MergeInput, inputs []*MergeInput) bool {
	if target.Index < 0 || target.Path != "" {
		return false
	}

	for _, input := range inputs {
		if input.Path != "" {
			return false
		}
	}
	return true
}

func mkMerge(op *pb.Op, target pb.InputIndex, inputs []*MergeInput) (*pb.Op, error) {
	merge := &pb.MergeOp{
		Inputs: make([]*pb.MergeInput, 0, len(inputs)+1),
	}
	merge.Inputs = append(merge.Inputs, &pb.MergeInput{
		Input: int64(target),
	})
	for _, input := range inputs {
		merge.Inputs = append(merge.Inputs, &pb.MergeInput{
			Input: int64(input.Index),
		})
	}

	op.Op = &pb.Op_Merge{Merge: merge}
	return op, nil
}

func resolvePath(fpath string) (inputPath, mountPath string, err error) {
	for inputPath = fpath; inputPath != "/"; {
		if _, err := os.Stat(filepath.Join(inputPath, "index.json")); err == nil {
			if mountPath == "" {
				mountPath = "/"
			}
			return inputPath, mountPath, nil
		} else if !os.IsNotExist(err) {
			return "", "", err
		}

		var prefix string
		inputPath, prefix = splitPath(inputPath)
		mountPath = "/" + prefix + mountPath
	}
	return "", "", errors.New("index.json not found")
}

func resolveInput(op *pb.Op, fpath string) (pb.InputIndex, string, error) {
	inputPath, mountPath, err := resolvePath(fpath)
	if err != nil {
		return -1, "", err
	}

	data, err := os.ReadFile(filepath.Join(inputPath, "index.json"))
	if err != nil {
		return -1, "", err
	}

	index := make(map[string]string)
	if err := json.Unmarshal(data, &index); err != nil {
		return -1, "", err
	}

	var sel string
	for path := range index {
		if strings.HasPrefix(mountPath, path) && len(path) > len(sel) {
			sel = path
		}
	}

	if sel == "" {
		return -1, "", fmt.Errorf("cannot find path %q in %s", mountPath, inputPath)
	}

	inputPath = index[sel]
	if sel != "/" {
		mountPath = strings.TrimPrefix(mountPath, sel)
	}

	i := slices.IndexFunc(op.Inputs, func(inp *pb.Input) bool {
		return inp.Digest == inputPath
	})
	if i < 0 {
		i = len(op.Inputs)
		op.Inputs = append(op.Inputs, &pb.Input{
			Digest: inputPath,
		})
	}
	return pb.InputIndex(i), mountPath, nil
}

func splitPath(s string) (dir, file string) {
	i := strings.LastIndex(s, "/")
	dir, file = s[:i], s[i+1:]
	if dir == "" {
		dir = "/"
	}
	return dir, file
}
