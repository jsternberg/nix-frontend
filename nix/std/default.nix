{ lib, config, ... }:

let
  minimal = rec {
    alpine = import ./alpine.nix { inherit lib config; };
    debian = import ./debian.nix { inherit lib config; };
    ubuntu = import ./ubuntu.nix { inherit debian config; };
  };

  args = {
    std = minimal;
    inherit lib config;
  };

  full = minimal // {
    golang = import ./golang args;
    rust = import ./rust args;
  };
in
full
