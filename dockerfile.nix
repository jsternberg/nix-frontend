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

        # Creates some bind mounts from the context and then runs heredoc script
        # to configure nix and copy the nix sources into the frontend.
        postSetup = lib.llb.run {
          mounts = {
            "/src/nix/channels/dockerfile".input = "${context}/nix/dockerfile";
            "/src/nix/channels/std".input = "${context}/nix/std";
            "/src/nix/profile/bin".input = "${context}/nix/bin";
            "/run".input = lib.llb.file null {
              "/setup".text = ''
                set -e
                echo 'filter-syscalls = false' >> /etc/nix/nix.conf
                nix-env -i $(nix-store --add /src/nix/channels) -p /nix/var/nix/profiles/per-user/root/channels
                nix-env -i $(nix-store --add /src/nix/profile) -p /nix/var/nix/profiles/per-user/root/profile
              '';
            };
          };
        } [ "/bin/sh" "/run/setup" ] baseImage;

        # Copy the binaries from the binaries target into the /bin folder.
        final = lib.llb.file postSetup {
          "/bin".source = project.binaries;
        };
      in
      final.override {
        # Set image entrypoint to /bin/frontend.
        meta.image.entrypoint = ["/bin/frontend"];
      };

    default = frontend;
  };
}
