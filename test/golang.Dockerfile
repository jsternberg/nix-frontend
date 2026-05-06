ARG ALPINE_VERSION=3.21
FROM alpine:${ALPINE_VERSION} AS alpine-base

FROM alpine-base AS verify-binaries
RUN --mount=type=bind,from=target-binaries,target=/fs <<EOT
  set -xe
  find /fs && [ -x /fs/sampleapp ]
EOT

FROM target-default AS verify-default
RUN <<EOT
  set -xe
  [ -x /bin/sampleapp ]
EOT
