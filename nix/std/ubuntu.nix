{ lib, config }:

let
  defaultConfig = {
    repository = "docker.io/library/ubuntu";
    version = "24.04";
  };
  cfg = defaultConfig // (config.ubuntu or {});

  image = {
    repository ? cfg.repository,
    version ? cfg.version,
  }:
  lib.llb.image "${repository}:${version}";

  baseImage = image {};
in
{
  inherit image;
  system = {
    image ? baseImage,
    systemPackages ? [],
  }:
  let
    packages = builtins.concatStringsSep " " systemPackages;

    installPackages = lib.optional (systemPackages != [])
      (lib.llb.run {
        env.DEBIAN_FRONTEND = "noninteractive";

        mounts."/var/lib/apt/lists".type = "tmpfs";
        mounts."/var/cache/apt".type = "tmpfs";
      } "apt-get update && apt-get install -y --no-install-recommends ${packages}");
  in
  installPackages image;
}
