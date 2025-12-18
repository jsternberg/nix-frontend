variable "ALPINE_VERSION" {
  default = null
}

variable "GO_VERSION" {
  default = null
}

target "docker-metadata-action" {}

target "_common" {
  args = {
    ALPINE_VERSION = ALPINE_VERSION
    GO_VERSION = GO_VERSION
  }
  dockerfile = "build.Dockerfile"
  inherits = ["docker-metadata-action"]
}

target "frontend" {
  inherits = ["_common"]
  tags = ["docker.io/jsternberg/dockerfile-nix"]
  target = "frontend"
}

target "libraries" {
  name = "library-${language}"
  matrix = {
    language = ["golang"]
  }
  target = "library-${language}"
  inherits = ["_common"]
  tags = ["docker.io/jsternberg/dockerfile-${language}"]
}

group "default" {
  targets = ["frontend", "libraries"]
}
