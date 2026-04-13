package dockerfile

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	"github.com/opencontainers/go-digest"
)

type graph struct {
	nodeByDigest map[digest.Digest]*graphNode
	digestOrder  []digest.Digest
}

type graphNode struct {
	g       *graph
	inputs  []*graphNode
	outputs []*graphNode
	dgst    digest.Digest
	op      *pb.Op
	meta    llb.OpMetadata
}

func newGraph(def *pb.Definition) (*graph, error) {
	g := &graph{
		nodeByDigest: make(map[digest.Digest]*graphNode),
		digestOrder:  make([]digest.Digest, 0, len(def.Def)),
	}
	for _, p := range def.Def {
		dgst := digest.FromBytes(p)
		g.digestOrder = append(g.digestOrder, dgst)

		var op pb.Op
		if err := op.Unmarshal(p); err != nil {
			return nil, err
		}

		node := &graphNode{
			g:    g,
			dgst: dgst,
			op:   &op,
		}
		if meta := def.Metadata[string(dgst)]; meta != nil {
			node.meta.FromPB(meta)
		}
		g.nodeByDigest[dgst] = node
	}

	// Link all of the inputs and outputs.
	for _, node := range g.nodeByDigest {
		for _, inp := range node.op.Inputs {
			inpNode, ok := g.nodeByDigest[digest.Digest(inp.Digest)]
			if !ok {
				return nil, fmt.Errorf("input digest %s for %s does not exist", inp.Digest, node.dgst)
			}
			inpNode.outputs = append(inpNode.outputs, node)
			node.inputs = append(node.inputs, inpNode)
		}
	}
	return g, nil
}

func (g *graph) Head() (digest.Digest, *pb.Op) {
	dgst := g.digestOrder[len(g.digestOrder)-1]
	return digest.Digest(dgst), g.nodeByDigest[dgst].op
}

func (g *graph) All() iter.Seq2[digest.Digest, *graphNode] {
	return func(yield func(digest.Digest, *graphNode) bool) {
		for _, dgst := range g.digestOrdering() {
			node := g.nodeByDigest[dgst]
			if !yield(node.dgst, node) {
				return
			}
		}
	}
}

func (g *graph) Walk(fn func(op *pb.Op) error) error {
	for _, n := range g.All() {
		if err := fn(n.op); err != nil {
			return err
		}
	}
	return nil
}

func (g *graph) ToDef() (*pb.Definition, error) {
	def := &pb.Definition{
		Metadata: make(map[string]*pb.OpMetadata),
	}
	digests := make(map[*graphNode]string)
	for _, dgst := range g.digestOrdering() {
		node := g.nodeByDigest[dgst]
		for i, inp := range node.inputs {
			node.op.Inputs[i].Digest = digests[inp]
		}

		p, err := node.op.Marshal()
		if err != nil {
			return nil, err
		}

		newDgst := string(digest.FromBytes(p))
		digests[node] = newDgst

		def.Def = append(def.Def, p)
		def.Metadata[newDgst] = node.meta.ToPB()
	}
	return def, nil
}

func (g *graph) digestOrdering() []digest.Digest {
	if g.digestOrder != nil {
		return g.digestOrder
	}

	var unvisited []*graphNode
	visited := make(map[*graphNode]struct{})

	digests := slices.Collect(maps.Keys(g.nodeByDigest))
	for _, dgst := range digests {
		if node := g.nodeByDigest[dgst]; len(node.inputs) == 0 {
			unvisited = append(unvisited, node)
		}
	}

	for len(unvisited) > 0 {
		node := unvisited[0]
		unvisited = unvisited[1:]

		g.digestOrder = append(g.digestOrder, node.dgst)
		visited[node] = struct{}{}

		for _, output := range node.outputs {
			if _, ok := visited[output]; ok {
				continue
			}

			canVisit := true
			for _, inp := range output.inputs {
				if _, ok := visited[inp]; !ok {
					canVisit = false
					break
				}
			}

			if canVisit {
				unvisited = append(unvisited, output)
			}
		}
	}
	return g.digestOrder
}

func (n *graphNode) Replace(ctx context.Context, st llb.State) error {
	def, err := st.Marshal(ctx)
	if err != nil {
		return err
	}

	if len(def.Def) < 2 {
		return errors.New("llb state is empty")
	} else if len(def.Def) == 2 {
		p := def.Def[0]

		var op pb.Op
		if err := op.Unmarshal(p); err != nil {
			return err
		}

		if len(op.Inputs) != len(n.inputs) {
			return errors.New("replacement node has a different number of inputs than the original")
		}

		if srcOp, ok := n.op.Op.(*pb.Op_Source); ok && len(srcOp.Source.Attrs) > 0 {
			if destOp, ok := op.Op.(*pb.Op_Source); ok {
				if destOp.Source.Attrs == nil {
					destOp.Source.Attrs = make(map[string]string)
				}
				maps.Copy(destOp.Source.Attrs, srcOp.Source.Attrs)
			}
		}
		n.op = &op

		mergedMeta := def.Metadata[digest.FromBytes(p)]
		if mergedMeta.Description == nil {
			mergedMeta.Description = n.meta.Description
		} else {
			maps.Copy(mergedMeta.Description, n.meta.Description)
		}
		n.meta = mergedMeta
		return nil
	}

	// Don't need a general solution to this at the moment since
	// this is only being used in this way on source nodes.
	if len(n.inputs) != 0 {
		return errors.New("replacing arbitrary state only works with no inputs")
	}

	n.g.digestOrder = nil
	for _, p := range def.Def {
		dgst := digest.FromBytes(p)

		var op pb.Op
		if err := op.Unmarshal(p); err != nil {
			return err
		}

		if srcOp, ok := n.op.Op.(*pb.Op_Source); ok && len(srcOp.Source.Attrs) > 0 {
			if destOp, ok := op.Op.(*pb.Op_Source); ok {
				if destOp.Source.Attrs == nil {
					destOp.Source.Attrs = make(map[string]string)
				}
				maps.Copy(destOp.Source.Attrs, srcOp.Source.Attrs)
			}
		}

		node := &graphNode{
			g:    n.g,
			dgst: dgst,
			op:   &op,
			meta: def.Metadata[dgst],
		}
		for _, inp := range op.Inputs {
			inpNode, ok := n.g.nodeByDigest[digest.Digest(inp.Digest)]
			if !ok {
				return fmt.Errorf("input digest %s for %s does not exist", inp.Digest, dgst)
			}
			inpNode.outputs = append(inpNode.outputs, node)
			node.inputs = append(node.inputs, inpNode)
		}
	}
	return nil
}
