{ lib, config }:

let
  defaultConfig = {
    repository = "docker.io/library/alpine";
    version = "3.21";
  };
  cfg = defaultConfig // (config.alpine or {});

  factoryFunc = {
    repository ? cfg.repository,
    version ? cfg.version,
  }: {
    image = lib.llb.image "${repository}:${version}";

    installSystemPackages = systemPackages:
      lib.llb.run (["apk" "add" "--no-cache"] ++ systemPackages);
  };
in
lib.system.makeFactory factoryFunc defaultConfig
