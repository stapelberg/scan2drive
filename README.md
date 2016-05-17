# scan2drive

<img src="https://github.com/stapelberg/scan2drive/raw/master/scan2drive.png"
width="266" align="right" alt="scan2drive screenshot">

scan2drive is a program (with a web interface) for converting and uploading
scanned documents to Google Drive, intended to be run on a small embedded
computer connected to a document scanner, such as a [Raspberry
Pi](https://www.raspberrypi.org/).

During the conversion step, scan2drive skips empty pages and converts the rest
from multi-megabyte JPEGs into a kilobyte-sized PDF. This allows you to use
Google Drive’s
[OCR](https://en.wikipedia.org/wiki/Optical_character_recognition)-based full
text search.

Both the originals and the converted PDF are uploaded to Google Drive, so that
you can enjoy full text search but still have the full-quality originals just
in case.

In comparison to the native Google Drive connectivity which some document
scanner vendors provide, scan2drive has these main advantages:

 1. scan2drive integrates with the scan button of your document scanner. You
    press one button and your documents will end up on Google Drive. Other
    solutions require you to use a mobile app or software on your PC.
 1. scan2drive is self-hosted and depends only on Google Drive being available,
    not the scanner vendor’s cloud integration service. Many vendors send
    documents into their own clouds and then to Google Drive. You are welcome
    to archive the scan directory of scan2drive to other places you see fit, in
    case there are any issues with Google Drive.
 1. scan2drive converts the scanned documents into a PDF which is small enough
    to be full text indexed by Google Drive, but it also retains the original
    JPEGs in case you need them.

## Project status and vision

Currently, there are a number of open issues and not all functionality might
work well. Use at your own risk!

The project vision is described above. Notably, scan2drive is already feature
complete. We don’t want to add any more features to it than it currently has.

scan2drive was published in the hope that it could be useful to others, but the
main author has no time to create an active community around it or accept
contributions in a timely manner. All support, development and bug fixes are
strictly best effort.

## Directory structure

The scans directory (`-scans_dir` flag) contains the following files:

 * `<sub>/` is the per-user directory under which scans are placed
  * `2016-05-09-21:05:02+0200/` is a directory for an individual scan
    * `page*.jpg` are the raw pages obtained by calling `scanimage`
    * `scan.pdf` is the converted PDF
    * `thumb.png` is the first page of the converted PDF for display in the UI
    * `COMPLETE.*` are empty files recording which individual processing steps
      are done

Any file in the scans directory can be deleted at will, with the caveat that
deleting scans before the `COMPLETE.uploadoriginals` file is present will
result in that scan being irrevocably lost.

The state directory (`-state_dir` flag) contains the following files:

 * `cookies.key` is a secret key with which cookies are encrypted
 * `sessions/` contains session contents
 * `users/` is a directory containing per-user data
  * `users/<sub>/` is a directory for an individual user
    * `drive_folder.json` contains information about the selected destination
      Google Drive folder. In case this file is deleted, the user will need to
      re-select the destination folder and scans cannot be uploaded until a new
      destination folder has been selected.
    * `token.json` contains the offline OAuth token for accessing Google Drive
      on behalf of the user. In case this file is deleted, the user will need
      to re-login. In case this file is leaked, the user should [revoke the
      token](https://security.google.com/settings/u/0/security/permissions)

## Dependencies

| Dependency | Min version | LKGV | Description |
| --- | --- | --- | --- |
| [SANE](http://www.sane-project.org/) | 1.0.25 | 1.0.25 | contains scanimage with libjpeg |
| [ImageMagick](https://www.imagemagick.org) | N/A | 6.8.9.9 | |
| [ScanTailor](http://scantailor.org/) | N/A | 0.9.11.1 | |
| [PDFBeads](https://rubygems.org/gems/pdfbeads/versions/1.1.1) | N/A | 1.1.1 | |

LKGV = Last known good version, i.e. a version with which I have successfully
run scan2drive.

## Installation

### Set up Raspberry Pi

Download the [Raspbian Jessie
Lite](https://www.raspberrypi.org/downloads/raspbian/) image and copy it to an
SD card. Insert the SD card into the Raspberry Pi and power it up.

Set a password, expand the root file system to use the entire SD card and
enable `/tmp` on RAM:

```bash
passwd
sudo raspi-config --expand-rootfs
sudo sed -i 's,^#RAMTMP=no$,RAMTMP=yes,g' /etc/default/tmpfs
reboot
```

### Install dependencies for scanning

As root, install SANE and scanbd:

```bash
apt-get update
apt-get install sane
addgroup pi scanner
apt-get install scanbd
echo 'net' | tee /etc/sane.d/dll.conf
echo -e 'connect_timeout = 3\nlocalhost' | tee /etc/sane.d/net.conf
cp -r /etc/sane.d/* /etc/scanbd/
echo 'fujitsu' | tee /etc/scanbd/dll.conf
systemctl enable scanbd.service
systemctl start scanbd.service
systemctl enable scanbm.socket
systemctl start scanbm.socket
systemctl stop inetd.service
systemctl disable inetd.service
```

On my machine, scanbd segfaults when a new device is attached (e.g. when the
scanner wakes up from sleep mode and gets enumerated on the USB bus), so I’ve
set up this workaround:
```bash
# mkdir /etc/systemd/system/scanbd.service.d
# cat >> /etc/systemd/system/scanbd.service.d/restart.conf <<'EOT'
[Service]
Restart=always
EOT
# systemctl daemon-reload
```

When pressing the scan button on your scanner, you should now get a new file in
`/tmp` matching `/tmp/scanbd.script.env.*`. This file is generated by
`/usr/share/scanbd/scripts/test.script`, which you should edit to run
scan2drive-trigger instead, like so:
```bash
su -s /bin/sh -c /usr/bin/scan2drive-trigger saned &
```

```bash
# for SANE ≥ 1.0.25 (scanimage with libJPEG support):
echo 'deb http://mirrordirector.raspbian.org/raspbian/ testing main contrib non-free rpi' | sudo tee /etc/apt/sources.list.d/testing.list
echo 'APT::Default-Release "stable";' | sudo tee /etc/apt/apt.conf.d/08default-release
sudo apt-get install -t testing sane-utils
```

### Install dependencies for conversion

The following steps need to be done on each machine which does conversion (the
Raspberry Pi and optionally another machine).

Install ImageMagick:
```bash
sudo apt-get install imagemagick
```

In case the Raspberry Pi is your only device doing conversion, it might be
worthwhile to recompile ImageMagick for ARM. I’ve observed the wall clock time
required for document conversion to drop from 2m11s to 1m55s after compiling
ImageMagick with `DEB_CFLAGS_MAINT_APPEND = -mcpu=cortex-a53
-mfpu=neon-fp-armv8 -mfloat-abi=hard`.

Install PDFBeads from Ruby Gems (because it is not packaged in Debian):
```bash
sudo gem install pdfbeads iconv
```

To achieve better compression, install [JBIG2enc](https://github.com/agl/jbig2enc):
```bash
sudo apt-get install libleptonica-dev
git clone https://github.com/agl/jbig2enc.git
cd jbig2enc
./autogen.sh
./configure
make -j8
sudo make install
```

Install ScanTailor:
```bash
sudo apt-get install scantailor
```

Until https://github.com/scantailor/scantailor/issues/178 is fixed, I’m compiling ScanTailor with the following patch:
```diff
--- i/filters/select_content/Task.cpp
+++ w/filters/select_content/Task.cpp
@@ -104,15 +104,6 @@ Task::process(TaskStatus const& status, FilterData const& data)
                ui_data.setDependencies(deps);
                ui_data.setMode(params->mode());
 
-               if (!params->dependencies().matches(deps)) {
-                       QRectF content_rect = ui_data.contentRect();
-                       QPointF new_center= deps.rotatedPageOutline().boundingRect().center();
-                       QPointF old_center = params->dependencies().rotatedPageOutline().boundingRect().center();
-
-                       content_rect.translate(new_center - old_center);
-                       ui_data.setContentRect(content_rect);
-               }
-
                if ((params->contentSizeMM().isEmpty() && !params->contentRect().isEmpty()) || !params->dependencies().matches(deps)) {
                        // Backwards compatibilty: put the missing data where it belongs.
                        Params const new_params(
```

### Install scan2drive

```bash
sudo apt-get install golang-go
export GOPATH=~/gocode
go get -u github.com/stapelberg/scan2drive/...
sudo cp ~/gocode/bin/scan2drive /usr/bin/scan2drive
sudo cp ~/gocode/bin/scan2drive-get-default-user /usr/bin/scan2drive-get-default-user
sudo cp ~/gocode/bin/scan2drive-process /usr/bin/scan2drive-process
cd ~/gocode/src/github.com/stapelberg/scan2drive
# In case you touched elements.html:
# sudo npm install -g vulcanize
# (cd static && vulcanize elements.html -o elements.vulcanized.html --inline-css --inline-scripts --strip-comments)
sudo cp scan2drive.service /etc/systemd/system
sudo cp examples/scan.sh /usr/bin/scan2drive-scan
sudo cp examples/trigger.sh /usr/bin/scan2drive-trigger
sudo mkdir -p /usr/share/scan2drive/static
sudo cp static/scan2drive.js static/elements.vulcanized.html static/drive48.png /usr/share/scan2drive/static
sudo systemctl daemon-reload
sudo systemctl restart scan2drive.service
sudo systemctl enable scan2drive.service
```
