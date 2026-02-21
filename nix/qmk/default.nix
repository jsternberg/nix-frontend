{ lib, std, ... }:

rec {
  firmware = {
    default = lib.llb.git "https://github.com/qmk/qmk_firmware.git";
    zsa = lib.llb.git "https://github.com/zsa/qmk_firmware.git#firmware25" // {
      v25 = lib.llb.git "https://github.com/zsa/qmk_firmware.git#firmware25";
    };
  };

  keyboard = {
    for,
    name,
    firmware ? firmware.default,
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

    defaultSource = lib.llb.local "context";
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
