package dockerfile

import (
	"encoding/json"
	"os"
)

type OpSpec struct {
	Source *SourceOp `json:"source,omitempty"`
	Exec   *ExecOp   `json:"exec,omitempty"`
	File   *FileOp   `json:"file,omitempty"`
	Merge  *MergeOp  `json:"merge,omitempty"`
	Meta   *Metadata `json:"meta,omitempty"`
}

type SourceOp struct {
	Identifier string            `json:"identifier"`
	Attributes map[string]string `json:"attrs,omitempty"`
}

type ExecOp struct {
	Command []string              `json:"command"`
	Mounts  map[string]*MountSpec `json:"mounts"`
	Workdir string                `json:"workdir"`
	Env     []string              `json:"env"`
}

type MountSpec struct {
	Type     string `json:"type,omitempty"`
	Input    string `json:"input,omitempty"`
	Readonly bool   `json:"readonly,omitempty"`
}

type FileOp struct {
	Target    string              `json:"target,omitempty"`
	Locations map[string]*FSEntry `json:"locations,omitempty"`
}

type FSEntry struct {
	Source string `json:"source,omitempty"`
	Text   string `json:"text,omitempty"`
	Mode   string `json:"mode,omitempty"`
}

type MergeOp struct {
	Target string   `json:"target,omitempty"`
	Inputs []string `json:"inputs,omitempty"`
}

type Metadata struct {
	Description map[string]string `json:"description,omitempty"`
}

func ReadOpSpec(fpath string) (*OpSpec, error) {
	in, err := os.ReadFile(fpath)
	if err != nil {
		return nil, err
	}

	var spec OpSpec
	if err := json.Unmarshal(in, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}
