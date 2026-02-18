{ lib, config, ... }:

rec {
  image = lib.llb.image "docker.io/library/golang:1.25-alpine3.21";
  build = {
    packages ? ["./cmd/..."],
    testPackages ? ["./..."],
    cgo ? false,
  }:
  let
    defaultMounts = {
      "/app".input = lib.llb.local "context";
      "/root/.cache/go-build".type = "cache";
      "/go/pkg/mod".type = "cache";
    };

    useXX = cgo && lib.platform.isCross;
    xx = (lib.llb.image "docker.io/tonistiigi/xx:1.9.0").override {
      platform = config.build;
    };

    targetPlatform = lib.platform.format config.target;

    mkBuildEnv = platform:
      let
        packages = lib.optional useXX (x: x ++ ["clang" "lld" "musl-dev" "pkgconfig"]) ["git"];
        step1 = lib.optional useXX
          (s: s.override { inherit platform; })
          image;
        step2 = lib.llb.run {} (["apk" "add" "--no-cache"] ++ packages) step1;
        step3 = lib.optional useXX (p: lib.llb.file p {
          "/".source = xx;
        }) step2;
      in
      step3;

    buildEnv = mkBuildEnv config.build;
    testEnv = mkBuildEnv config.target;

    goBin = if useXX then "xx-go" else "go";
    buildCommand = [ goBin "build" "-o" "/out" ] ++ packages;
    doBuild = lib.llb.run {
      env.CGO_ENABLED = if useXX then "1" else "0";
      env.TARGETPLATFORM = targetPlatform;
      workdir = "/app";

      mounts = defaultMounts // {
        "/out" = {};
      };
    } buildCommand;

    buildStage = (doBuild buildEnv).override {
      meta.description."llb.customname" = "go build ${builtins.concatStringsSep " " packages}";
    };

    testCommand = [
      "/run/bin/gotestsum"
      "--junitfile=/out/unit-tests.xml"
      "--format=standard-verbose"
    ] ++ testPackages;
    doTest = lib.llb.run {
      env.CGO_ENABLED = if useXX then "1" else "0";
      workdir = "/app";

      mounts = defaultMounts // {
        "/run/bin".input = testSupportBinaries;
        "/out" = {};
      };
    } testCommand;

    testStage = doTest testEnv;

    gotestsum = let
      version = "1.13.0";
      pkgpath = "gotest.tools/gotestsum";
      command = [goBin "install" "${pkgpath}@v${version}"];

      installStage = lib.llb.run {
        env.GOBIN = "/out";

        mounts = defaultMounts // {
          "/out" = {};
        };
      } command buildEnv;
    in installStage.override {
      meta.description."llb.customname" = "go install gotest.tools/gotestsum@v${version}";
    };

    testSupportBinaries = lib.llb.merge null [ "${gotestsum}/out" ];

    vendorCommand = [ "/bin/sh" "/run/vendor" ];
    doVendor = lib.llb.run {
      env.CGO_ENABLED = if useXX then "1" else "0";
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

    vendorStage = doVendor buildEnv;

    validateVendorCommand = ["diff" "-u" "/a" "/b"];
    doValidateVendor = lib.llb.run {
      mounts = {
        "/a".input = (lib.llb.local "context").override {
          attrs.followpaths = ["go.mod" "go.sum" "vendor"];
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
