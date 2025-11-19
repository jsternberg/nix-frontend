{
  system ? builtins.currentSystem,
  argsfile ? null,
  configuration,
}:

let
  lib = {
    llb = import ./llb { inherit system; };
    optional = x: y: if x then y else (x: x);
  };

  args = if argsfile != null
    then builtins.fromJSON argsfile
    else {};

  allArgs = args // {
    inherit lib args;
    config = config.config or {};
  };
  config = let
    f = import configuration;
    filteredArgs = builtins.intersectAttrs (builtins.functionArgs f) args;
  in
    f filteredArgs;

  inputs = let
    f = config.inputs or (x: {});
  in
    f allArgs;

  mappedInputs = let
    mapped = builtins.mapAttrs (name: lib.llb.marshal) inputs;
  in
    lib.llb.inputs mapped;

  targets = let
    f = config.targets;
    inputNames = builtins.attrNames inputs;
    importByName = name: {
      inherit name;
      value = import (builtins.findFile builtins.nixPath name) allArgs;
    };
    defaultImports.std = import <std> allArgs;
    userImports = builtins.listToAttrs (builtins.map importByName inputNames);
    withImports = allArgs
      // defaultImports
      // userImports;
    targets = f withImports;
  in
    builtins.mapAttrs (name: lib.llb.marshal) targets;

  finalConfig = config // {
    inherit targets;
    inputs = mappedInputs;
  };
in
{
  config.build = finalConfig;
}
