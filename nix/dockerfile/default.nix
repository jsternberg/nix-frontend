{
  system ? builtins.currentSystem,
  configuration ? null,
  dockerfile,
}:

let
  args = if configuration != null
    then builtins.fromJSON (builtins.readFile configuration)
    else {};

  buildArgs = args.build-arg or {};

  lib = (import ./utils.nix) // {
    llb = import ./llb {
      inherit lib system args;
      config = mergedConfig;
    };
    platform = import ./platform.nix mergedConfig;
    system = import ./system.nix lib;
  };

  mkDefaultConfig = {
    buildOs ? null,
    buildOsVersion ? null,
    buildArch ? null,
    buildVariant ? null,
    targetOs ? null,
    targetOsVersion ? null,
    targetArch ? null,
    targetVariant ? null,
    targetStage ? "default",
    ...
  }:
  let
    build = {
      os = buildOs;
      osVersion = buildOsVersion;
      arch = buildArch;
      variant = buildVariant;
    };

    target = {
      os = targetOs;
      osVersion = targetOsVersion;
      arch = targetArch;
      variant = targetVariant;
      stage = targetStage;
    };
  in
  {
    inherit build target;
  };

  mergedConfig = (mkDefaultConfig buildArgs) // (config.config or {});
  allArgs = buildArgs // {
    inherit lib;
    args = buildArgs;
    config = mergedConfig;
  };
  config = let
    f = import dockerfile;
    filteredArgs = builtins.intersectAttrs (builtins.functionArgs f) buildArgs;
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
      value = import (builtins.findFile builtins.nixPath name) withImports;
    };
    defaultImports.std = import <std> allArgs;
    userImports = builtins.listToAttrs (builtins.map importByName inputNames);

    withImports = allArgs // defaultImports;
    withAllImports = withImports // userImports;
    targets = f withAllImports;
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
