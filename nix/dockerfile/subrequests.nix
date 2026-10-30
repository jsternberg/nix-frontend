{ config, lib }:

{
  targets =
    let
      inputs = config.targets or { };
      names = builtins.attrNames inputs;
      result.targets = builtins.map (name: {
        inherit name;
      }) names;
    in
    lib.writeFile "targets.json" (builtins.toJSON result);
}
