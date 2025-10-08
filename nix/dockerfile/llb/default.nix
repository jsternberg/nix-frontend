{ system ? builtins.currentSystem }:

{
  marshal = input: derivation {
    name = "llb-def.json";
    inherit system input;
    builder = "/bin/marshal";
    args = [ "$input" "$out" ];
  };
}
