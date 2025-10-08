package dockerfile

import (
	"iter"

	"github.com/moby/buildkit/solver/pb"
	"github.com/opencontainers/go-digest"
)

type graph struct {
	opByDigest  map[string]*pb.Op
	digestOrder []string
	metadata    map[string]*pb.OpMetadata
}

func newGraph(def *pb.Definition) (*graph, error) {
	var (
		opByDigest  = make(map[string]*pb.Op)
		digestOrder []string
	)
	for _, p := range def.Def {
		dgst := string(digest.FromBytes(p))
		digestOrder = append(digestOrder, dgst)

		op := &pb.Op{}
		if err := op.Unmarshal(p); err != nil {
			return nil, err
		}
		opByDigest[dgst] = op
	}
	return &graph{
		opByDigest:  opByDigest,
		digestOrder: digestOrder,
		metadata:    def.Metadata,
	}, nil
}

func (g *graph) Head() (digest.Digest, *pb.Op) {
	dgst := g.digestOrder[len(g.digestOrder)-1]
	return digest.Digest(dgst), g.opByDigest[dgst]
}

func (g *graph) All() iter.Seq2[digest.Digest, *pb.Op] {
	return func(yield func(digest.Digest, *pb.Op) bool) {
		for _, dgst := range g.digestOrder {
			op := g.opByDigest[dgst]
			if !yield(digest.Digest(dgst), op) {
				return
			}
		}
	}
}

func (g *graph) Walk(fn func(op *pb.Op) error) error {
	for _, op := range g.All() {
		if err := fn(op); err != nil {
			return err
		}
	}
	return nil
}

func (g *graph) ToDef() (*pb.Definition, error) {
	def := &pb.Definition{
		Metadata: make(map[string]*pb.OpMetadata),
	}
	newDigests := make(map[string]string)
	for _, dgst := range g.digestOrder {
		op := g.opByDigest[dgst]
		for _, inp := range op.Inputs {
			if newDgst, ok := newDigests[inp.Digest]; ok {
				inp.Digest = newDgst
			}
		}

		p, err := op.Marshal()
		if err != nil {
			return nil, err
		}

		newDgst := string(digest.FromBytes(p))
		if newDgst != dgst {
			newDigests[dgst] = newDgst
		}
		def.Def = append(def.Def, p)
		if meta := g.metadata[dgst]; meta != nil {
			def.Metadata[newDgst] = meta
		}
	}
	return def, nil
}
