{ lib, std, ... }:

let
  default = lib.llb.git "https://github.com/qmk/qmk_firmware.git";
  zsa = default.override {
    url = "https://github.com/zsa/qmk_firmware.git";
    ref = "firmware25";
  };
in
{
  firmware = {
    inherit default;
    zsa = zsa // {
      firmware25 = zsa;
      firmware24 = zsa.override {
        ref = "firmware24";
      };
    };
  };

  keyboard = {
    for,
    name,
    firmware ? default,
    source ? null,
  }:
  let
    target = "${for}:${name}";

    # todo: this is pretty big for a base image.
    # can probably see if this can be made smaller.
    base = std.debian.slim.setup {
      systemPackages = [
        "python3-full"
        "build-essential"
        "gcc-arm-none-eabi"
        "libnewlib-arm-none-eabi"
        "zstd"
        "clang-format"
        "diffutils"
        "unzip"
        "zip"
        "libhidapi-hidraw0"
        "dfu-util"
        "git"
      ];
    };

    qmk-base = lib.llb.run "python3 -m venv /opt/qmk && /opt/qmk/bin/pip install qmk && ln -s /opt/qmk/bin/qmk /usr/local/bin/qmk" base;

    defaultSource = lib.llb.context;
    userSource = if source != null then source else defaultSource;

    buildStep = lib.llb.run {
      workdir = "/root/qmk_firmware";
      env.QMK_BIN = "/opt/qmk/bin/qmk";
      mounts = {
        "/root/qmk_firmware/keyboards/${for}/keymaps/${name}".input = userSource;
        "/root/qmk_firmware".input = firmware;
        "/root/qmk_firmware/.build" = {
          type = "cache";
          sharing = "locked";
        };
        "/run".input = lib.llb.file null {
          "/make".text = ''
            #/bin/sh
            set -ex
            make QMK_BIN=$QMK_BIN "$1"

            mkdir -p "$2"
            cp *.bin "$2"/
          '';
        };
        "/out" = {};
      };
    } "/bin/sh /run/make ${target} /out" qmk-base;
  in
  "${buildStep}/out";
}
