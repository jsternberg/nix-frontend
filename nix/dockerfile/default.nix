{
  system ? builtins.currentSystem,
  argsfile ? null,
  configuration,
}:

let
  lib = {
    llb = import ./llb { inherit system; };
  };

  args = if argsfile != null
    then builtins.fromJSON argsfile
    else {};

  allArgs = args // { inherit lib args; };
  config = let
    f = import configuration;
    filteredArgs = builtins.intersectAttrs (builtins.functionArgs f) args;
  in
    f filteredArgs;

  inputs = let
    f = config.inputs or (x: {});
    inputs = f allArgs;
    mapped = builtins.mapAttrs (name: lib.llb.marshal) inputs;
  in
    lib.llb.inputs mapped;

  targets = let
    f = config.targets;
    inputs = f allArgs;
  in
    builtins.mapAttrs (name: lib.llb.marshal) inputs;

  finalConfig = config // { inherit inputs targets; };
in
{
  config.build = finalConfig;
}
