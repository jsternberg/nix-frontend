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

target "docker-metadata-action" {}

target "_common" {
  args = {
    REPOSITORY = REPOSITORY
    VERSION = VERSION
    ALPINE_VERSION = ALPINE_VERSION
    GO_VERSION = GO_VERSION
  }
  dockerfile = "build.Dockerfile"
  inherits = ["docker-metadata-action"]
}

target "frontend" {
  inherits = ["_common"]
  tags = ["docker.io/jsternberg/dockerfile-nix:${VERSION}"]
  target = "frontend"
}

target "libraries" {
  name = "library-${language}"
  matrix = {
    language = ["golang"]
  }
  target = "library-${language}"
  inherits = ["_common"]
  tags = ["docker.io/jsternberg/dockerfile-${language}:${VERSION}"]
}

group "default" {
  targets = ["frontend", "libraries"]
}
