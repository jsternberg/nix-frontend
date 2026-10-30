system:

rec {
  optional = x: y: if x then y else (x: x);

  makeOverridable =
    f: origArgs:
    let
      origRes = f origArgs;

      overrideWith =
        newArgs: origArgs // (if builtins.isFunction newArgs then newArgs origArgs else newArgs);
    in
    origRes
    // {
      override = newArgs: makeOverridable f (overrideWith newArgs);
    };

  writeFile =
    name: text:
    derivation {
      inherit name system text;
      builder = "/bin/sh";
      args = [
        "-c"
        ''/bin/mv "$textPath" "$out"''
      ];
      passAsFile = [ "text" ];
    };
}
