{ lib, ... }:

rec {
  image = lib.llb.image "docker.io/library/golang:1.25-alpine3.21";
  build = {
    packages ? ["./cmd/..."],
    testPackages ? ["./..."],
  }:
  let
    defaultMounts = {
      "/app".input = lib.llb.local "context" {};
      "/root/.cache/go-build".type = "cache";
      "/go/pkg/mod".type = "cache";
    };

    # TODO: should be able to reuse the definition in alpine here.
    toolBuildEnv = lib.llb.run {} ["apk" "add" "--no-cache" "git"] image;

    buildCommand = [ "go" "build" "-o" "/out" ] ++ packages;
    doBuild = lib.llb.run {
      env.CGO_ENABLED = "0";
      workdir = "/app";

      mounts = defaultMounts // {
        "/out" = {};
      };

      meta.description = {
        "llb.customname" = "go build ${builtins.concatStringsSep " " packages}";
      };
    } buildCommand;

    buildStage = doBuild image;

    testCommand = [
      "/run/bin/gotestsum"
      "--junitfile=/out/unit-tests.xml"
      "--format=standard-verbose"
    ] ++ testPackages;
    doTest = lib.llb.run {
      env.CGO_ENABLED = "0";
      workdir = "/app";

      mounts = defaultMounts // {
        "/run/bin".input = testSupportBinaries;
        "/out" = {};
      };

      meta.description = {
        "llb.customname" = "go test ${builtins.concatStringsSep " " testPackages}";
      };
    } testCommand;

    testStage = doTest image;

    gotestsum = let
      version = "1.13.0";
      pkgpath = "gotest.tools/gotestsum";
      command = ["go" "install" "${pkgpath}@v${version}"];
    in lib.llb.run {
      env.GOBIN = "/out";

      mounts = defaultMounts // {
        "/out" = {};
      };

      meta.description = {
        "llb.customname" = "go install gotest.tools/gotestsum@v${version}";
      };
    } command toolBuildEnv;

    testSupportBinaries = lib.llb.merge null [ "${gotestsum}/out" ];

    vendorCommand = [ "/bin/sh" "/run/vendor" ];
    doVendor = lib.llb.run {
      env.CGO_ENABLED = "0";
      workdir = "/app";

      mounts = defaultMounts // {
        "/run".input = lib.llb.file null {
          "/vendor".text = ''
            #/bin/sh
            set -eo pipefail
            go mod tidy

            go mod vendor -o /out/vendor
            cp go.mod go.sum /out/
          '';
        };
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
    test = "${testStage}/out";
    vendor = "${vendorStage}/out";
    validate-vendor = validateVendorStage;
  in
  {
    inherit binaries test vendor validate-vendor;
    default = binaries;
  };
}
