# syntax=docker.io/jsternberg/dockerfile-nix:local

{ goVersion ? null, alpineVersion ? null }:

{
  config = {
    alpine.version = "3.20";
  };

  targets = { lib, std, ... }:
  let
    targets = std.golang.build {};
  in
  targets // (with targets; {
    frontend = std.alpine.system {
      systemPackages = ["curl"];
    };
  });
}
