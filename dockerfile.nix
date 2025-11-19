# syntax=docker.io/jsternberg/dockerfile-nix

{ goVersion ? null, alpineVersion ? null }:

{
  inputs = { lib, ... }: {
    golang = lib.llb.image "docker.io/jsternberg/dockerfile-golang:latest";
  };

  config = {
    alpine.version = "3.20";
  };

  targets = { lib, std, golang, ... }:
  let
    targets = golang.build {};
  in
  targets // (with targets; {
    frontend = std.alpine.system {
      systemPackages = ["curl"];
    };
  });
}
