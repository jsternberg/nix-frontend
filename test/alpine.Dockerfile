FROM target-default AS verify-system-packages
RUN <<EOT
  set -xe
  which -a curl
EOT

FROM target-override AS verify-override-image
RUN <<EOT
  set -xe
  which -a curl
  which -a wget
  grep "VERSION_ID=3.20" < /etc/os-release
EOT
