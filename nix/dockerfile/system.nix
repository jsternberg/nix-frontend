lib:

let
  makeOverridable = f: origArgs:
    let
      origRes = f origArgs;
      overrideWith = newArgs: origArgs //
        (if builtins.isFunction newArgs
          then newArgs origArgs
          else newArgs);
    in
    origRes // {
      override = newArgs: makeOverridable f (overrideWith newArgs);
    };

  fromFunc = f: config:
    let
      impl = {
        image,
        installSystemPackages,
        destdir ? "",
      }: {
        _type = "system";

        system = {
          systemPackages ? [],
          packages ? [],
        }:
        let
          step1 = lib.optional (systemPackages != []) (installSystemPackages systemPackages) image;

          # TODO: I want to reimplement this in a more robust way.
          # Gonna keep this implementation for now.
          byPrefix = builtins.groupBy (x: x.meta.installPrefix) packages;
          installPackageFuncs = builtins.attrValues
            (builtins.mapAttrs
              (prefix: x: y: lib.llb.merge "${y}${destdir}${prefix}" x)
              byPrefix);
          installPackages = lib.optional (packages != [])
            (x: builtins.foldl' (acc: f: f acc) x installPackageFuncs);

          step2 = installPackages step1;
        in
        step2;
      };
    in
    impl (f config);
in
{
  makeFactory = f: makeOverridable (fromFunc f);
}
