#!/bin/bash

set -x

cd $(mktemp -d)
git clone /usr/src/scan2drive
cd scan2drive
export PATH=$PWD/_bundled_turbojpeg:$PATH
GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc go install '-ldflags=-linkmode external -extldflags -static' -tags turbojpeg -v ./cmd/scan2drive
