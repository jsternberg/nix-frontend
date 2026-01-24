# syntax=docker.io/jsternberg/dockerfile-nix:local

{ goVersion ? "1.25", alpineVersion ? "3.21" }:

{
  config = {
    # TODO: should be ok to set this to null to use the default
    # but that's not presently possible.
    alpine.version = alpineVersion;
    # TODO: currently this option is ignored and the golang
    # builder doesn't respect the alpine version.
    go.version = goVersion;
  };

  targets = { lib, std, ... }:
  let
    targets = std.golang.build {};
    context = lib.llb.local "context";
  in
  targets // {
    frontend =
      let
        baseImage = std.alpine.system {
          systemPackages = ["nix"];
        };

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
      in
      (lib.llb.file postSetup {
        "/bin".source = targets.default;
      }).override {
        meta.image.entrypoint = ["/bin/frontend"];
      };
  };
}
