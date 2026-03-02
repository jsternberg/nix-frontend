# syntax=docker.io/jsternberg/dockerfile-nix:local

{ goVersion ? "1.25", alpineVersion ? null }:

{
  config = {
    # TODO: should be ok to set this to null to use the default
    # but that's not presently possible.
    ${if alpineVersion != null then "alpine.version" else null} = alpineVersion;

    # TODO: currently this option is ignored and the golang
    # builder doesn't respect the alpine version.
    go.version = goVersion;
  };

  # Define targets for this dockerfile.
  targets = { lib, std, ... }:
  let
    # Use cgo when building go project. Use normal defaults.
    project = std.golang.build { cgo = true; };
    context = lib.llb.local "context";
  in
  rec {
    # Only include binaries and test targets.
    # We do not use vendoring so there's no point in including
    # those targets here.
    inherit (project) binaries test;

    # Build for the frontend.
    # We use some custom steps because we make some manual strange changes
    # after installing the base system and copying the binaries.
    frontend =
      let
        # Base step uses the current alpine version and installs the
        # nix binary.
        baseImage = std.alpine.setup {
          systemPackages = ["nix"];
        };

        # Copy the binaries from the binaries target into the /bin folder
        # and also copy the nix files into the appropriate directories.
        final = lib.llb.file baseImage {
          "/nix/var/nix/dockerfile".source = "${context}/nix/dockerfile";
          "/nix/var/nix/std".source = "${context}/nix/std";
          "/bin".source = lib.llb.merge null [ "${context}/nix/bin" project.binaries ];
          "/etc/nix/nix.conf".text = ''
            nix-path = /nix/var/nix
            filter-syscalls = false
          '';
        };
      in
      final.override {
        # Set image entrypoint to /bin/frontend.
        meta.image.entrypoint = ["/bin/frontend"];
      };

    default = frontend;
  };
}
