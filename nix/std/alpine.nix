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
    systemPackages ? [],
  }: {
    image = lib.llb.image "${repository}:${version}";

    installSystemPackages = extraSystemPackages:
      let
        allSystemPackages = systemPackages ++ extraSystemPackages;
        impl = lib.llb.run (["apk" "add" "--no-cache"] ++ allSystemPackages);
      in
      lib.optional (allSystemPackages != []) impl;
  };
in
lib.system.makeFactory factoryFunc defaultConfig
