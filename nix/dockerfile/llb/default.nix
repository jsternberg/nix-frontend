{ system ? builtins.currentSystem }:

let
  mkOp = name: spec: derivation {
    name = "llb-${name}";
    inherit system;
    builder = "/bin/mkop";
    args = [ "$specPath" "$out" ];

    passAsFile = ["spec"];
    spec = builtins.toJSON spec;
  };

  toAttrStr = v: if builtins.isString v
    then v
    else builtins.toJSON v;
in
rec {
  local = name: attrs: mkOp "source" {
    source = {
      identifier = "local://${name}";
      attrs = builtins.mapAttrs (k: toAttrStr) attrs;
    };
  };

  image = name: mkOp "source" {
    source.identifier = "docker-image://${name}";
  };

  merge = target: inputs: mkOp "merge" {
    merge = { inherit target inputs; };
  };

  file = target: locations: mkOp "file" {
    file = { inherit target locations; };
  };

  run = optsOrCommand: let
      make = {
        mounts ? {},
        env ? {},
        workdir ? "/",
        meta ? {},
      }: command: input: mkOp "exec" {
        exec = {
          command = if builtins.isString command
            then [ "/bin/sh" "-c" command ]
            else command;
          mounts = {
            "/".input = input;
          } // mounts;
          inherit workdir;
          env = builtins.attrValues (builtins.mapAttrs (name: value: "${name}=${value}") env);
        };
        inherit meta;
      };
    in
      if (builtins.isList optsOrCommand || builtins.isString optsOrCommand)
        then make {} optsOrCommand
        else make optsOrCommand;

  marshal = input: derivation {
    name = "llb-def.json";
    inherit system;
    input = merge input [];
    builder = "/bin/marshal";
    args = [ "$input" "$out" ];
  };
}
