# syntax=docker.io/jsternberg/dockerfile-nix

{ goVersion ? null, alpineVersion ? null }:

{
  inputs = { lib, ... }: {
    golang = lib.llb.image "docker.io/jsternberg/dockerfile-golang:latest";
  };

  config = {
    alpine.version = "3.20";
  };

  targets = { lib, golang, ... }:
  let
    targets = golang.build {};
  in
  targets;
}
