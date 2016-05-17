#!/bin/sh
set -e
RELDIR=$(date +%F-%T%z)
USER=$(scan2drive-get-default-user)
DIR=~/scans/${USER}/${RELDIR}
echo "${RELDIR}"
mkdir -p "${DIR}"
# Delete the scan directory in case scanimage fails.
cleanup() {
	[ $? -eq 0 ] || rmdir "${DIR}"
}
trap cleanup EXIT
cd "${DIR}"
scanimage \
  --format=jpeg \
  --resolution 600dpi \
  --source 'ADF Duplex' \
  --page-width 210mm --page-height 297mm \
  -x 210mm -y 297mm \
  --mode=Color \
  --batch=page%d.jpg
touch COMPLETE.scan
sync
