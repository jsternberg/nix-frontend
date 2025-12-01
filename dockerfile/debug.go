package dockerfile

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/solver/pb"
	"google.golang.org/protobuf/encoding/protojson"
)

func buildDebugOutput(ctx context.Context, c client.Client, def *pb.Definition) (*client.Result, error) {
	var jsonDef struct {
		Def      []json.RawMessage         `json:"def"`
		Metadata map[string]*pb.OpMetadata `json:"metadata"`
	}

	jsonDef.Metadata = def.Metadata
	for _, b := range def.Def {
		var op pb.Op
		if err := op.Unmarshal(b); err != nil {
			return nil, err
		}

		src, err := protojson.Marshal(&op)
		if err != nil {
			return nil, err
		}
		jsonDef.Def = append(jsonDef.Def, json.RawMessage(src))
	}

	out, err := json.Marshal(jsonDef)
	if err != nil {
		return nil, err
	}

	var dst bytes.Buffer
	if err := json.Indent(&dst, out, "", "  "); err != nil {
		return nil, err
	}

	st := llb.Scratch().File(
		llb.Mkfile("def.json", 0o644, dst.Bytes()),
	)
	debugDef, err := st.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	req := client.SolveRequest{
		Definition: debugDef.ToPB(),
	}
	return c.Solve(ctx, req)
}
