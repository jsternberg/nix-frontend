config@{ build, target, ... }:

let
  isEmptyOrNull = x: x == null || x == "";
in
rec {
  format = {os, arch, variant ? null, ...}:
    if isEmptyOrNull os
      then "unknown"
      else builtins.concatStringsSep "/" (builtins.filter (x: !(isEmptyOrNull x)) [os arch variant]);

  equal = x: y:
    let
      xs = format x;
      ys = format y;
    in
    xs == ys && xs != "unknown";

  isCross = ! equal build target;
}
