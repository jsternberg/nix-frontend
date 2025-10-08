package main

import (
	"os"

	"github.com/jsternberg/nix-frontend/dockerfile"
	"github.com/moby/buildkit/frontend/gateway/grpcclient"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/sirupsen/logrus"
)

func main() {
	if err := grpcclient.RunFromEnvironment(appcontext.Context(), dockerfile.Build); err != nil {
		logrus.Errorf("fatal error: %+v", err)
		os.Exit(1)
	}
}
