#!/bin/sh
set -e
USER=$(scan2drive-get-default-user)
RELDIR=$(scan2drive-scan)
scan2drive-process --user=${USER} "${RELDIR}"
