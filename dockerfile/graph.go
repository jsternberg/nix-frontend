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
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/opencontainers/go-digest"
)

type graph struct {
	nodeByDigest map[digest.Digest]*graphNode
	digestOrder  []digest.Digest
	head         *graphNode
}

type graphNode struct {
	g       *graph
	inputs  []*graphNode
	outputs []*graphNode
	dgst    digest.Digest
	op      *pb.Op
	meta    llb.OpMetadata
	image   *dockerspec.DockerOCIImage
}

func newGraph(def *pb.Definition) (*graph, error) {
	g := &graph{
		nodeByDigest: make(map[digest.Digest]*graphNode),
		digestOrder:  make([]digest.Digest, 0, len(def.Def)),
	}

	for i, p := range def.Def {
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

		if i == len(def.Def)-1 {
			g.head = node
		}
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
	if g.head == nil || len(g.head.inputs) == 0 {
		return "", nil
	}
	last := g.head.inputs[0]
	return last.dgst, last.op
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

func (g *graph) Roots() iter.Seq2[digest.Digest, *graphNode] {
	return func(yield func(digest.Digest, *graphNode) bool) {
		for dgst, n := range g.All() {
			if len(n.op.Inputs) == 0 {
				if !yield(dgst, n) {
					return
				}
			}
		}
	}
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

	unvisited := []*graphNode{g.head}
	visited := make(map[*graphNode]struct{})

	for len(unvisited) > 0 {
		node := unvisited[0]
		unvisited = unvisited[1:]

		g.digestOrder = append(g.digestOrder, node.dgst)
		visited[node] = struct{}{}

		for _, inp := range node.inputs {
			// Check if all outputs for this input have been visited.
			// If they have, we can visit this node.
			canVisit := true
			for _, out := range inp.outputs {
				if _, ok := visited[out]; !ok {
					canVisit = false
					break
				}
			}

			if canVisit {
				unvisited = append(unvisited, inp)
			}
		}
	}
	slices.Reverse(g.digestOrder)
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

	// Reset the digest order.
	n.g.digestOrder = nil

	head, err := def.Head()
	if err != nil {
		return err
	}

	// Create the new nodes from this definition.
	nodeByDigest := make(map[digest.Digest]*graphNode)
	for _, p := range def.Def[:len(def.Def)-1] {
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

		var node *graphNode
		if dgst == head {
			// Reuse the node we're replacing so the inputs and outputs
			// for other nodes remain valid.
			node = n
			node.inputs = nil
		} else {
			node = &graphNode{g: n.g}
		}
		node.dgst = dgst
		node.op = &op
		node.meta = def.Metadata[dgst]
		nodeByDigest[dgst] = node
	}

	// Link the new digests.
	for dgst, node := range nodeByDigest {
		for _, inp := range node.op.Inputs {
			inpNode, ok := nodeByDigest[digest.Digest(inp.Digest)]
			if !ok {
				return fmt.Errorf("input digest %s for %s does not exist", inp.Digest, dgst)
			}
			inpNode.outputs = append(inpNode.outputs, node)
			node.inputs = append(node.inputs, inpNode)
		}
	}
	maps.Copy(n.g.nodeByDigest, nodeByDigest)
	return nil
}
