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
{ myBuildArg ? null, myOtherBuildArg ? null }:

{
  inputs = { lib, ... }:
  {
    golang = lib.llb.image "docker.io/jsternberg/dockerfile-golang";
  };

  targets = { std, golang, ... }:
  let
    project = golang.build {};
  in
  {
    // Only interested in the binaries target.
    inherit (project) binaries;
    default = std.alpine.system {
      systemPackages = ["ca-certificates"];
      packages = [binaries];
    };
  }
}
```

Imported inputs may be from any source supported by Buildkit. This may be from an image, a git repository, an http source, or even your local context. This library will then be injected as an argument to the function defined on `targets` and usable within the dockerfile.
