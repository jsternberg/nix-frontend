# Dockerfile Nix Frontend

This is the code for the nix-powered dockerfile frontend.
It is a custom buildkit frontend that uses nix to generate dockerfiles using the low-level builder (LLB) format.

## Usage

Use `docker buildx bake` to produce the frontend image, the runner helper image, and the nix libraries for various languages.

```sh
$ docker buildx bake
```

Add the following to the top of a file called `dockerfile.nix`.

```nix
# syntax=jsternberg/dockerfile-nix
```

Run your build.

```sh
$ docker buildx build -f dockerfile.nix .
```

The `--target` flag can be used to build different targets. The `default` target is used by default.

## File Syntax

The `dockerfile.nix` file is the definition of your build. It is of the format:

```nix
{ myBuildArg ? null, myOtherBuildArg ? null }:

{
  targets = { std, ... }:
  {
    default = std.alpine.system {
      systemPackages = [ "curl" ];
    };
  };
}
```

Other build files written in Nix may also be injected to the script through the `inputs` parameter.

```nix
# Build arguments. Arguments are converted from ALL_CAPS to camelCase so myBuildArg is MY_BUILD_ARG on
# the command line.
#
# To elide build arguments, you can do _: at the top of the file.
{ myBuildArg ? null, myOtherBuildArg ? null }:

{
  # Optional dependencies. The contents will be injected into the parameter list for targets.
  inputs = { lib, ... }:
  {
    # An example of what an input might look like. This input doesn't exist so uncommenting
    # this won't work. Any LLB can be used here so files from the local context, git repositories,
    # or images may be used as inputs.
    # golang = lib.llb.image "docker.io/jsternberg/dockerfile-golang";
  };

  # Declared targets. Minimum parameters passed are lib, config, and std modules.
  # Any additional dependencies declared in inputs will also be passed. If golang
  # were defined above, golang would be included in the list of targets.
  #
  # The ... allows us to not include any parameters we don't use so we could
  # remove lib and config from this parameter list since they aren't used in the body.
  targets = { lib, config, std, ... }:
  let
    # Invoke the build function from std.golang. This creates a golang project
    # with all of the default values.
    #
    # This is just a variable and isn't exposed as a target.
    project = std.golang.build {};
  in
  {
    # Only interested in the binaries target.
    # std.golang.build defines many targets that might be useful. We are only interested in the
    # binaries target so we use inherit to expose it here.
    # This code is the same as if we did `binaries = project.binaries;`.
    inherit (project) binaries;

    # Define a default target that installs the ca-certificates package inside of alpine
    # with apk add and then copies the binaries from the binaries target into the image.
    default = std.alpine.system {
      systemPackages = ["ca-certificates"];
      packages = [binaries];
    };
  }
}
```

Imported inputs may be from any source supported by Buildkit. This may be from an image, a git repository, an http source, or even your local context. This library will then be injected as an argument to the function defined on `targets` and usable within the dockerfile.
