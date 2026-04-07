{ lib, config, std, ... }:

let
  rust-version = "1.93.1";
in
rec {
  alpine = std.alpine.override ({version, ...}: {
    repository = "docker.io/library/rust";
    version = "${rust-version}-alpine${version}";
  });

  build = {
    system ? alpine,
    profile ? "release",
  }:
    let
      buildEnv = system.setup {};

      app = (lib.llb.context).override {
        # Common to accidentally have the target directory from a local invocation
        # so just exclude it here in case it is present.
        attrs."local.excludepatterns" = [ "target" ];
      };

      env = {
        CARGO_HOME = "/cargo";
        CARGO_BUILD_BUILD_DIR = "/build";
        CARGO_BUILD_TARGET_DIR = "/target";
      };
      workdir = "/app";

      defaultMounts = {
        "/app".input = app;

        "/build" = {
          type = "cache";
          # The build directory isn't writeable by multiple processes so mark the sharing
          # as locked to avoid duplicate writes at the same time.
          sharing = "locked";
        };

        # Cache directories for downloaded dependencies.
        "/cargo/git".type = "cache";
        "/cargo/registry".type = "cache";
      };

      buildCommand = if profile == "release"
        then
          [ "cargo" "build" "--release" ]
        else if profile == "debug" then
          [ "cargo" "build" ]
        else
          [ "cargo" "build" "--profile=${profile}" ];

      doBuild = lib.llb.run {
        inherit env workdir;
        mounts = defaultMounts // {
          "/target/${profile}" = {};
        };
      } buildCommand;

      buildStage = (doBuild buildEnv);
      binaries = lib.llb.file null {
        "/" = {
          source = "${buildStage}/target/${profile}";
          exclude = ["examples" "*.d"];
        };
      }
      // { meta.installPrefix = "/bin"; };

      doLockfileCommand = command: lib.llb.run {
        inherit env workdir;
        mounts = defaultMounts;
      } [ "cargo" command ] buildEnv;

      lockfileStage = update: let
        command = if update then "update" else "generate-lockfile";
        result = doLockfileCommand command;
      in
      lib.llb.file null {
        "Cargo.lock".source = "${result}/app/Cargo.lock";
      };

      generate-lockfile = lockfileStage false;
      update-lockfile = lockfileStage true;
    in
    {
      default = binaries;
      inherit binaries generate-lockfile update-lockfile;
    };
}
