# syntax=jsternberg/dockerfile-nix

{ goVersion ? null, alpineVersion ? null }:

{
  config = {
    alpine.version = "3.20";
  };

  targets = { lib, ... }: {
    default = lib.llb.image "docker.io/library/ubuntu";
  };
}
