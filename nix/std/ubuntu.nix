{ debian, config }:

let
  defaultConfig = {
    repository = "docker.io/library/ubuntu";
    version = "24.04";
  };
  cfg = defaultConfig // (config.ubuntu or {});
in
debian.override cfg
