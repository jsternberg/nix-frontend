# syntax=docker.io/jsternberg/dockerfile-nix:local

{ goVersion ? "1.25" }:

{
  # config = {
  #   go.version = goVersion;
  # };

  targets = { std, ... }:
  let
    project = std.golang.build {};
  in
  rec {
    # Expose the binaries and test targets produced by the
    # standard golang builder.
    inherit (project) binaries test;

    # Default target is the binaries image.
    default = (std.alpine.setup { packages = [ binaries ]; }).override {
      meta.image.entrypoint = ["/bin/sampleapp"];
    };
  };
}
