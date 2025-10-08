package dockerfile

import (
	"context"
	"errors"

	"github.com/moby/buildkit/frontend/gateway/client"
)

func Build(ctx context.Context, c client.Client) (*client.Result, error) {
	return nil, errors.New("implement me")
}
