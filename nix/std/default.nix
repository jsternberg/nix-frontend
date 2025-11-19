{ lib, config, ... }:

{
  alpine = let
    defaultConfig = {
      repository = "docker.io/library/alpine";
      version = "3.21";
    };
    cfg = defaultConfig // (config.alpine or {});

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
      packages ? [],
    }:
    let
      installSystemPackages = lib.optional (systemPackages != [])
        (lib.llb.run (["apk" "add" "--no-cache"] ++ systemPackages));

      byPrefix = builtins.groupBy (x: x.meta.installPrefix) packages;
      installPackageFuncs = builtins.attrValues
        (builtins.mapAttrs
          (prefix: x: y: lib.llb.merge "${y}/usr${prefix}" x)
          byPrefix);
      installPackages = lib.optional (packages != [])
        (x: builtins.foldl' (acc: f: f acc) x installPackageFuncs);
    in
    installPackages (installSystemPackages image);
  };

  ubuntu = let
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
  };
}
