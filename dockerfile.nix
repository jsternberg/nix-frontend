# syntax=docker.io/jsternberg/dockerfile-nix

{ goVersion ? null, alpineVersion ? null }:

{
  inputs = { lib, ... }: {
    golang = lib.llb.image "docker.io/jsternberg/dockerfile-golang";
  };

  config = {
    alpine.version = "3.20";
  };

  targets = { lib, ... }:
  let
    golangImage = lib.llb.image "docker.io/library/golang:1.25-alpine3.21";

    packages = [ "./cmd/..." ];
    buildCommand = [ "go" "build" "-o" "/out" ] ++ packages;
    doBuild = lib.llb.run {
      env.CGO_ENABLED = "0";
      workdir = "/app";

      mounts = {
        "/app".input = lib.llb.local "context" {};
        "/root/.cache/go-build".type = "cache";
        "/go/pkg/mod".type = "cache";
        "/out" = {};
      };

      meta.description = {
        "llb.customname" = "go build ${builtins.concatStringsSep " " packages}";
      };
    } buildCommand;

    buildStage = doBuild golangImage;
  in
  rec {
    binaries = {
      outPath = "${buildStage}/out";
      meta.installPrefix = "/bin";
    };

    default = binaries;
  };
}
