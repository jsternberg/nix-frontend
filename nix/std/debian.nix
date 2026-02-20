{ lib, config }:

let
  defaultConfig = {
    repository = "docker.io/library/debian";
    version = "12.13";
  };
  cfg = defaultConfig // (config.debian or {});

  factoryFunc = {
    repository,
    version,
  }: {
    image = lib.llb.image "${repository}:${version}";

    installSystemPackages = systemPackages:
      let
        packages = builtins.concatStringsSep " " systemPackages;
      in
      lib.llb.run {
        env.DEBIAN_FRONTEND = "noninteractive";

        mounts."/var/lib/apt/lists".type = "tmpfs";
        mounts."/var/cache/apt".type = "tmpfs";
      } "apt-get update && apt-get install -y --no-install-recommends ${packages}";
  };

  base = lib.system.makeFactory factoryFunc cfg;
in
base // {
  slim = base.override { version, ... }: {
    version = "${version}-slim";
  };
}
