## Overview

Produces a build for a Go project using the standard Go tooling. A simple project with the default targets can be created with the following:

```nix
_:

{
  targets = { std, ... }:
    std.golang.build {};
}
```

## Example

```nix
_:

{
  targets = { std, ... }:
    let
      project = std.golang.build {
        # Binary packages to build as part of the `binaries` target.
        # packages = ["./cmd/..."];

        # Test packages to include as part of the `test` target.
        # It would be possible to split testing of different packages into different targets
        # by creating multiple golang builds with different test package patterns and then
        # rearranging the final targets into different names. Most commonly, you just want to test
        # the entire project.
        # testPackages = ["./..."];

        # Enable cgo when building. The golang package always uses cross compilation but enabling
        # cgo will also install the compiler toolchain for cross compiling cgo code. This defaults
        # to disabling cgo.
        # cgo = false;

        # Enable vendor targets. This will add the `vendor` and `validate-vendor` targets.
        # `vendor` will produce the contents of the `vendor` folder with `go mod vendor` and is intended to be
        # used with `-o type=local,dest=vendor/`. `validate-vendor` is meant to be used with CI to ensure
        # the vendor directory, `go.mod`, and `go.sum` files are correct.
        # vendor = false;
      };

      image = std.alpine.setup {
        # Install the produces build packages into the `/bin` directory of an alpine image.
        packages = [ project.binaries ];
      };
    in
    # Expose all targets from the golang build.
    # If `vendor` was set to `true` this would also include `vendor` and `validate-vendor`.
    # If we wanted to be more selected, we could also use `inherit` instead of the merge operator (`//`) like below:
    #     inherit (project) test;
    project // {
      # Expose the alpine image we created with our binaries.
      inherit image;

      # Overwrite the default target from project with the image we created.
      default = image;
    };
  }
```
