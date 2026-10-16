{ lib, config }:

let
  defaultConfig = {
    repository = "docker.io/library/debian";
    version = "12.13";
  };
  cfg = defaultConfig // (config.debian or { });

  factoryFunc =
    {
      repository,
      version,
      platform ? null,
      systemPackages ? [ ],
    }:
    {
      image = lib.optional (platform != null) (x: x.override { inherit platform; }) (
        lib.llb.image "${repository}:${version}"
      );

      installSystemPackages =
        extraSystemPackages:
        let
          allSystemPackages = systemPackages ++ extraSystemPackages;
          packages = builtins.concatStringsSep " " allSystemPackages;
          impl = lib.llb.run {
            env.DEBIAN_FRONTEND = "noninteractive";

            mounts."/var/lib/apt/lists".type = "tmpfs";
            mounts."/var/cache/apt".type = "tmpfs";
          } "apt-get update && apt-get install -y --no-install-recommends ${packages}";
        in
        lib.optional (allSystemPackages != [ ]) impl;
    };

  base = lib.system.makeFactory factoryFunc cfg;
in
base
// {
  slim = base.override (
    { version, ... }: {
      version = "${version}-slim";
    }
  );
}
