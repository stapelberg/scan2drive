# scan2drive

<img src="https://github.com/stapelberg/scan2drive/raw/main/scan2drive.png"
width="266" align="right" alt="scan2drive screenshot">

scan2drive is a Go program (with a web interface) for scanning, converting and
uploading physical documents to Google Drive. The author runs scan2drive as a
[gokrazy](https://gokrazy.org/) appliance on a Raspberry Pi 4.

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

## Supported scanners

* scan2drive can scan from **any AirScan-compatible scanner**. This means any
  scanner that is marketed as compatible with Apple iPhones should work. You can
  find a list of tested devices at
  https://github.com/stapelberg/airscan#tested-devices
* Fujitsu ScanSnap iX500 connected via USB

## Architecture

![](/img/2021-11-14-scan2drive-architecture.svg)

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

## Installation

First, [install gokrazy](https://gokrazy.org/quickstart.html).

Then, pack the `github.com/stapelberg/scan2drive/cmd/scan2drive` Go package.

As an example, assuming your SD card is accessible as `/dev/sdx`:

```
gokr-packer -overwrite=/dev/sdx github.com/stapelberg/scan2drive/cmd/scan2drive
```

Boot your Raspberry Pi 3 from this SD card and connect the Fujitsu
ScanSnap iX500 document scanner via USB (no other scanner is
supported).

You should be able to access the gokrazy web interface at the URL
which `gokr-packer` printed. To access the scan2drive web interface,
switch to port 7120.

## Building with libjpeg-turbo

[libjpeg-turbo](https://libjpeg-turbo.org/) is a JPEG image codec that uses SIMD
instructions (Arm Neon in case of the Raspberry Pi) to accelerate baseline JPEG
compression.

scan2drive can optionally make use of libjpeg-turbo (via the `turbojpeg` build
tag), but doesn’t include it by default because of the cumbersome setup.

Using libjpeg-turbo on gokrazy requires a few extra setup steps. Because gokrazy
does not include a C runtime environment (neither libc nor a dynamic linker), we
need to link scan2drive statically.

1. Install the gcc cross compiler, for example on Debian:
    ```
   apt install crossbuild-essential-arm64
   ```

1. Enable cgo for your gokrazy instance. This means setting the following
   environment variables when calling `gokr-packer`:

    ```
    export CC=aarch64-linux-gnu-gcc
    export CGO_ENABLED=1
    ```

1. Make available libjpeg-turbo. You have two options for this step.

   * Option A: Use the bundled version in the scan2drive repository by including its
   `pkg-config` wrapper in your `PATH` environment variable:
        ```
        PATH=/home/michael/go/src/github.com/stapelberg/scan2drive/_bundled_turbojpeg:$PATH
        ```

   * Option B: Install libjpeg-turbo for arm64, for example on Debian:
        ```
		dpkg --add-architecture arm64
		apt update
	    apt install libturbojpeg0-dev:arm64 libjpeg62-turbo-dev:arm64
		export PKG_CONFIG_PATH=/usr/lib/aarch64-linux-gnu/pkgconfig
	    ```
     Ensure that you have the statically linked version available (`libturbojpeg.a`),
	 not just the dynamically linked version (`libturbojpeg.so`).

1. Enable static linking and the `turbojpeg` build tag for scan2drive:

    ```
    mkdir -p buildflags/github.com/stapelberg/scan2drive/cmd/scan2drive
    echo '-ldflags=-linkmode external -extldflags -static' > buildflags/github.com/stapelberg/scan2drive/cmd/scan2drive/buildflags.txt

    mkdir -p buildtags/github.com/stapelberg/scan2drive/cmd/scan2drive
    echo turbojpeg > buildtags/github.com/stapelberg/scan2drive/cmd/scan2drive/buildtags.txt
    ```
