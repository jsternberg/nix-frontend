{ lib, ... }:

rec {
  image = lib.llb.image "docker.io/library/golang:1.25-alpine3.21";
  build = {
    packages ? ["./cmd/..."],
  }:
  let
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

    buildStage = doBuild image;

    vendorCommand = [ "/bin/sh" "/run/vendor" ];
    doVendor = lib.llb.run {
      env.CGO_ENABLED = "0";
      workdir = "/app";

      mounts = {
        "/app".input = lib.llb.local "context" {};
        "/run".input = lib.llb.file null {
          "/vendor".text = ''
            #/bin/sh
            set -eo pipefail
            go mod tidy

            go mod vendor -o /out/vendor
            cp go.mod go.sum /out/
          '';
        };
        "/root/.cache/go-build".type = "cache";
        "/go/pkg/mod".type = "cache";
        "/out" = {};
      };
    } vendorCommand;

    vendorStage = doVendor image;

    validateVendorCommand = ["diff" "-u" "/a" "/b"];
    doValidateVendor = lib.llb.run {
      mounts = {
        "/a".input = lib.llb.local "context" {
          followpaths = ["go.mod" "go.sum" "vendor"];
        };
        "/b".input = "${vendorStage}/out";
      };
    } validateVendorCommand;

    validateVendorStage = doValidateVendor (lib.llb.image "docker.io/library/alpine:3.21");

    binaries = {
      outPath = "${buildStage}/out";
      meta.installPrefix = "/bin";
    };
    vendor = "${vendorStage}/out";
    validate-vendor = validateVendorStage;
  in
  {
    inherit binaries vendor validate-vendor;
    default = binaries;
  };
}
