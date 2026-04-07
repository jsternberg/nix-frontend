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

target "_test_runner" {
  contexts = {
    "jsternberg/dockerfile-nix:local" = "target:frontend"
  }
}

target "test-alpine-runner" {
  name = "test-alpine-runner-${tgt}"
  inherits = ["_test_runner"]
  dockerfile = "dockerfile.nix"
  context = "test/alpine"

  matrix = {
    tgt = ["default", "override"]
  }
  target = tgt
}

target "test-alpine" {
  name = "test-alpine-test-${test}"
  contexts = {
    target-default = "target:test-alpine-runner-default"
    target-override = "target:test-alpine-runner-override"
  }
  dockerfile = "test/alpine.Dockerfile"
  output = ["type=cacheonly"]

  matrix = {
    test = [
      "verify-system-packages",
      "verify-override-image"
    ]
  }
  target = test
}

group "test" {
  targets = ["test-alpine"]
}
