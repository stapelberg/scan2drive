#!/bin/bash

set -x

export PATH=$PWD/_bundled_turbojpeg:$PATH
GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc go install -ldflags='-linkmode external -extldflags -static' -tags turbojpeg github.com/stapelberg/scan2drive
