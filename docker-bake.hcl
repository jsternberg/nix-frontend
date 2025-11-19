variable "REPOSITORY" {
  default = "docker.io/jsternberg/dockerfile-nix"
}

variable "VERSION" {
  default = "latest"
}

variable "ALPINE_VERSION" {
  default = null
}

variable "GO_VERSION" {
  default = null
}

target "_common" {
  args = {
    REPOSITORY = REPOSITORY
    VERSION = VERSION
    ALPINE_VERSION = ALPINE_VERSION
    GO_VERSION = GO_VERSION
  }
  dockerfile = "build.Dockerfile"
}

target "nix-runner" {
  inherits = ["_common"]
  tags = ["docker.io/jsternberg/dockerfile-nix:${VERSION}-nix"]
  target = "nix-runner"
}

target "frontend" {
  inherits = ["_common"]
  tags = ["docker.io/jsternberg/dockerfile-nix:${VERSION}"]
  target = "frontend"
}

target "libraries" {
  inherits = ["_common"]
  tags = ["docker.io/jsternberg/dockerfile-golang:${VERSION}"]
  target = "library-golang"
}

group "default" {
  targets = ["nix-runner", "frontend", "libraries"]
}
