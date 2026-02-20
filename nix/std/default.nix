{ lib, config, ... }:

let
  alpine = import ./alpine.nix { inherit lib config; };
  debian = import ./debian.nix { inherit lib config; };
  ubuntu = import ./ubuntu.nix { inherit debian config; };

  minimal = {
    inherit alpine ubuntu;
  };
  args = {
    std = minimal;
    inherit lib config;
  };

  full = minimal // {
    golang = import ./golang args;
  };
in
full
