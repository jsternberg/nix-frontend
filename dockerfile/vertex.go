package dockerfile

import (
	"encoding/json"

	"github.com/moby/buildkit/solver/pb"
	"google.golang.org/protobuf/encoding/protojson"
)

type Vertex struct {
	Op   *pb.Op    `json:"op"`
	Meta *Metadata `json:"meta,omitempty"`
}

type vertex struct {
	Op   json.RawMessage `json:"op"`
	Meta *Metadata       `json:"meta,omitempty"`
}

func (v Vertex) MarshalJSON() ([]byte, error) {
	out := vertex{
		Meta: v.Meta,
	}

	var err error
	if out.Op, err = protojson.Marshal(v.Op); err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

func (v *Vertex) UnmarshalJSON(p []byte) error {
	var in vertex
	if err := json.Unmarshal(p, &in); err != nil {
		return err
	}

	v.Op = new(pb.Op)
	if err := protojson.Unmarshal(in.Op, v.Op); err != nil {
		return err
	}
	v.Meta = in.Meta
	return nil
}
