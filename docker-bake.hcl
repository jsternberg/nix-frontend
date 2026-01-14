variable "ALPINE_VERSION" {
  default = null
}

variable "GO_VERSION" {
  default = null
}

target "meta-helper" {
  tags = ["docker.io/jsternberg/dockerfile-nix:local"]
}

target "_common" {
  args = {
    ALPINE_VERSION = ALPINE_VERSION
    GO_VERSION = GO_VERSION
  }
  dockerfile = "build.Dockerfile"
}

target "frontend" {
  inherits = ["_common", "meta-helper"]
  target = "frontend"
}

group "default" {
  targets = ["frontend"]
}
