{
  system ? builtins.currentSystem,
  argsfile ? null,
  configuration,
}:

let
  lib = {
    llb = import ./llb {
      inherit system;
      config = mergedConfig;
    };
    optional = x: y: if x then y else (x: x);
  };

  args = if argsfile != null
    then builtins.fromJSON (builtins.readFile argsfile)
    else {};

  mkDefaultConfig = {
    buildPlatform ? "unknown/unknown",
    buildOs ? "unknown",
    buildOsVersion ? null,
    buildArch ? "unknown",
    buildVariant ? null,
    targetPlatform ? "unknown/unknown",
    targetOs ? "unknown",
    targetOsVersion ? null,
    targetArch ? "unknown",
    targetVariant ? null,
    targetStage ? "default",
    ...
  }: {
    build = {
      platform = buildPlatform;
      os = buildOs;
      osVersion = buildOsVersion;
      arch = buildArch;
      variant = buildVariant;
    };

    target = {
      platform = targetPlatform;
      os = targetOs;
      osVersion = targetOsVersion;
      arch = targetArch;
      variant = targetVariant;
      stage = targetStage;
    };
  };

  mergedConfig = (mkDefaultConfig args) // (config.config or {});
  allArgs = args // {
    inherit lib args;
    config = mergedConfig;
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
