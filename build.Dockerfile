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

FROM scratch AS library-golang
COPY ./nix/golang /

FROM alpine:${ALPINE_VERSION} AS alpine-base
FROM alpine-base AS frontend
RUN apk add --no-cache nix
RUN --mount=target=/src/nix/channels/dockerfile,source=./nix/dockerfile \
    --mount=target=/src/nix/channels/std,source=./nix/std \
    --mount=target=/src/nix/profile/bin,source=./nix/bin <<EOT
  set -e
  nix-env -i $(nix-store --add /src/nix/channels) -p /nix/var/nix/profiles/per-user/root/channels
  nix-env -i $(nix-store --add /src/nix/profile) -p /nix/var/nix/profiles/per-user/root/profile
EOT
COPY --from=binaries . /bin/
ENTRYPOINT ["/bin/frontend"]
