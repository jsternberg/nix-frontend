{
  config,
  system ? builtins.currentSystem,
}:

let
  targetPlatform = {
    inherit (config.target) arch os osVersion variant;
  };

  hasPlatform = x: x != null && x ? platform;

  mkOp = name: f:
  let
    mkDerivation = spec: derivation {
      name = "llb-${name}";
      inherit system;
      builder = "/bin/mkop";
      args = [ "$specPath" "$out" ];

      passAsFile = ["spec"];
      spec = builtins.toJSON spec;
    };

    makeOverridable = f: origArgs@{
      platform ? targetPlatform,
      meta ? null,
      ...
    }:
    let
      args = builtins.intersectAttrs (builtins.functionArgs f) origArgs;
      origRes = mkDerivation (
        (f args)
        // (if meta != null then { inherit meta; } else {})
        // { inherit platform; }
      );
    in
    origRes // {
      override = newArgs:
        let
          args = (if builtins.isFunction newArgs
            then newArgs origArgs
            else newArgs) // origArgs;
        in
        makeOverridable f args;

      # Make available for inheriting.
      inherit platform;
    };
  in
  makeOverridable f;

  mkSource = mkOp "source" ({
    identifier,
    attrs ? null,
  }: {
    source = {
      inherit identifier;
      ${if attrs != null then "attrs" else null} = builtins.mapAttrs (k: toAttrStr) attrs;
    };
  });

  mkMerge = mkOp "merge" ({
    target,
    inputs,
  }: {
    merge = { inherit target inputs; };
  });

  mkFile = mkOp "file" ({
    target,
    locations,
  }: {
    file = { inherit target locations; };
  });

  mkExec = mkOp "exec" ({
    command,
    mounts,
    workdir ? null,
    env ? {},
    meta ? {},
  }: {
    exec = {
      command = if builtins.isString command
        then [ "/bin/sh" "-c" command ]
        else command;
      inherit meta mounts;
      workdir = if workdir != null
        then workdir
        else "/";
      env = builtins.attrValues (builtins.mapAttrs (name: value: "${name}=${value}") env);
    };
  });

  toAttrStr = v: if builtins.isString v
    then v
    else builtins.toJSON v;
in
rec {
  local = name: mkSource {
    identifier = "local://${name}";
  };

  git = url: mkSource {
    identifier = "git://${url}";
  };

  image = name: mkSource {
    identifier = "docker-image://${name}";
  };

  merge = target: inputs: mkMerge {
    inherit target inputs;
    ${if hasPlatform target then "platform" else null} = target.platform;
  };

  file = target: locations: mkFile {
    inherit target locations;
    ${if hasPlatform target then "platform" else null} = target.platform;
  };

  run = optsOrCommand:
    if (builtins.isList optsOrCommand || builtins.isString optsOrCommand)
      then input: mkExec {
        command = optsOrCommand;
        mounts."/".input = input;
        ${if hasPlatform input then "platform" else null} = input.platform;
      }
      else command: input:
        let
          base = mkExec {
            inherit command;
            ${if hasPlatform input then "platform" else null} = input.platform;
          };
          overrideArgs = optsOrCommand // {
            mounts = {
              "/".input = input;
            } // (optsOrCommand.mounts or {});
          };
        in
        base.override overrideArgs;

  inputs = spec: derivation {
    name = "llb-inputs.json";
    inherit system;
    builder = "/bin/readinputs";
    args = [ "$specPath" "$out" ];

    spec = builtins.toJSON spec;
    passAsFile = ["spec"];
  };

  marshal = input: derivation {
    name = "llb-def.json";
    inherit system;
    input = merge input [];
    builder = "/bin/marshal";
    args = [ "$input" "$out" ];
  };
}
