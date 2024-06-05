#!/bin/bash

set -x

cd $(mktemp -d)
git config --global --add safe.directory /usr/src/
git clone /usr/src/scan2drive
cd scan2drive
GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc go install '-ldflags=-linkmode external -extldflags -static' -tags turbojpeg -v ./cmd/scan2drive
