{ lib, config }:

let
  defaultConfig = {
    repository = "docker.io/library/alpine";
    version = "latest";
  };
  cfg = defaultConfig // (config.alpine or { });

  factoryFunc =
    {
      repository ? cfg.repository,
      version ? cfg.version,
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
          impl = lib.llb.run (
            [
              "apk"
              "add"
              "--no-cache"
            ]
            ++ allSystemPackages
          );
        in
        lib.optional (allSystemPackages != [ ]) impl;
    };
in
lib.system.makeFactory factoryFunc defaultConfig
