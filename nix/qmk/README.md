## Overview

Allows compiling a keyboard firmware with the QMK toolchain.

```nix
_:

{
  inputs = { lib, ... }:
  {
    qmk = lib.llb.git {
      url = "https://github.com/jsternberg/nix-frontend";
      subdir = "nix/qmk";
    };
  };

  targets = { lib, qmk, ... }:
  {
    # Used to build a keyboard firmware.
    # Uncommented lines are required arguments.
    default = qmk.keyboard {
      # Path to the keyboard. Maps to the location of the specific keyboard in
      # https://github.com/qmk/qmk_firmware/master/keyboards/<for>
      for = "zsa/moonlander";

      # Keymap name. This affects the firmware binary output name.
      # The keyboard source will be mounted into the firmware repository
      # in a directory that matches this name.
      name = "jsternberg";

      # Keyboard firmware to use. Defaults to the qmk/qmk_firmware repository but
      # an alias also exists for using zsa's fork. Any fork of qmk_firmware will work.
      # firmware = qmk.firmware.default;

      # Where to retrieve the keymap source from. This defaults to the main context
      # directory.
      # source = lib.llb.context "context";
    };
  };
}
```

### Using the ZSA fork

This is an example of using this builder to build a ZSA Moonlander keyboard firmware using the output of the download source option in Oryx Configurator.

```nix
_:

{
  inputs = { lib, ... }:
  {
    qmk = lib.llb.git {
      url = "https://github.com/jsternberg/nix-frontend";
      subdir = "nix/qmk";
    };
  };

  targets = { lib, qmk, ... }:
  {
    default = qmk.keyboard {
      # Keyboard names are different in the ZSA fork.
      for = "zsa/moonlander/reva";

      # The name of my configuration.
      name = "jsternberg";

      # Using the ZSA fork with the most recent firmware.
      # If a specific branch is desired, there are other aliases such as v25 or the
      # git source can be overridden or used directly to customize the options.
      firmware = qmk.firmware.zsa;
    };
  };
}
```
