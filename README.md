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
