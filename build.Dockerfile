ARG ALPINE_VERSION=3.21
ARG GO_VERSION=1.25

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS gobuild-base
WORKDIR /app

FROM gobuild-base AS dockerfile-version
ARG REPOSITORY
ARG VERSION
RUN <<EOT
  set -ex
  export PKG=github.com/jsternberg/nix-frontend/dockerfile
  [ -n "$REPOSITORY" ] && echo " -X ${PKG}.Repository=${REPOSITORY}" >> /tmp/.ldflags
  [ -n "$VERSION" ] && echo " -X ${PKG}.Version=${VERSION}" >> /tmp/.ldflags
EOT

FROM gobuild-base AS dockerfile
RUN --mount=target=. \
    --mount=target=/go/pkg/mod,type=cache \
    --mount=target=/root/.cache/go-build,type=cache \
    --mount=target=/tmp/.ldflags,source=/tmp/.ldflags,from=dockerfile-version <<EOT
  set -ex
  mkdir -p /out
  go build -ldflags "$(cat /tmp/.ldflags)" -o /out/ ./cmd/...
EOT

FROM scratch AS binaries
COPY --from=dockerfile /out/ /

FROM alpine:${ALPINE_VERSION} AS alpine-base

FROM alpine-base AS nix-runner
RUN apk add --no-cache nix
COPY --from=binaries /marshal /mkop /readinputs /bin/
RUN --mount=target=/src/nix/channels/dockerfile,source=./nix/dockerfile \
    --mount=target=/src/nix/channels/std,source=./nix/std \
    --mount=target=/src/nix/profile/bin,source=./nix/bin <<EOT
  set -e
  nix-env -i $(nix-store --add /src/nix/channels) -p /nix/var/nix/profiles/per-user/root/channels
  nix-env -i $(nix-store --add /src/nix/profile) -p /nix/var/nix/profiles/per-user/root/profile
EOT
ENV PATH="/nix/var/nix/profiles/per-user/root/profile/bin:$PATH"

FROM scratch AS library-golang
COPY ./nix/golang /

FROM alpine-base AS frontend
COPY --from=binaries frontend /bin/
ENTRYPOINT ["/bin/frontend"]
