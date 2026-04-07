ARG ALPINE_VERSION=3.21
ARG GO_VERSION=1.25

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS gobuild-base
WORKDIR /app

FROM gobuild-base AS dockerfile
RUN --mount=target=. \
    --mount=target=/go/pkg/mod,type=cache \
    --mount=target=/root/.cache/go-build,type=cache <<EOT
  set -ex
  mkdir -p /out
  go build -o /out/ ./cmd/...
EOT

FROM scratch AS binaries
COPY --from=dockerfile /out/ /

FROM alpine:${ALPINE_VERSION} AS alpine-base
FROM alpine-base AS frontend
LABEL moby.buildkit.frontend.caps="moby.buildkit.frontend.inputs,moby.buildkit.frontend.contexts,moby.buildkit.frontend.subrequests"
RUN apk add --no-cache nix
COPY ./nix/dockerfile /nix/var/nix/dockerfile
COPY ./nix/std /nix/var/nix/std
COPY ./nix/bin /bin/
COPY <<EOT /etc/nix/nix.conf
nix-path = /nix/var/nix
filter-syscalls = false
EOT
COPY --from=binaries . /bin/
ENTRYPOINT ["/bin/frontend"]
