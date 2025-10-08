# syntax=jsternberg/dockerfile-nix

{ goVersion ? null, alpineVersion ? null }:

{
  config = {
    alpine.version = "3.20";
  };

  targets = { lib, ... }: {
    default = ./sample.json;
  };
}
