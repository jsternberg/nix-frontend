# syntax=docker.io/jsternberg/dockerfile-nix:local

_:

{
  targets = { std, ... }:
  {
    default = std.alpine.setup {
      systemPackages = [ "curl" "ca-certificates" ];
    };

    override =
      let
        alpine = std.alpine.override {
          version = "3.20";
          systemPackages = [ "curl" ];
        };
      in
        alpine.setup {
          systemPackages = [ "wget" ];
        };
  };
}
