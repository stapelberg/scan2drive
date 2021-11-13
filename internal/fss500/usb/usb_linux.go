// Copyright 2016 Michael Stapelberg and contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package usb is a minimal, device-specific library which uses
// Linux’s usbdevfs and /sys interfaces to communicate with a Fujitsu
// ScanSnap iX500 via USB.
package usb

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/unix"
)

// TODO: move UsbdevfsBulkTransfer and USBDEVFS_* constants to x/sys/unix
type usbdevfsBulkTransfer struct {
	Ep        uint32
	Len       uint32
	Timeout   uint32
	Pad_cgo_0 [4]byte
	Data      *byte
}

const (
	uSBDEVFS_BULK             = 0xc0185502
	uSBDEVFS_CLAIMINTERFACE   = 0x8004550f
	uSBDEVFS_RELEASEINTERFACE = 0x80045510
)

const usbDevicesRoot = "/sys/bus/usb/devices"

// Constants specific to the Fujitsu ScanSnap iX500
const (
	// product is the USB product id for the ScanSnap iX500
	product = "132b"

	// vendor is Fujitsu’s USB vendor ID
	vendor = "04c5"

	// deviceToHost is the USB endpoint used to transfer data from the
	// device to the host
	deviceToHost = 129

	// hostToDevice is the USB endpoint used to transfer data from the
	// host to the device
	hostToDevice = 2
)

// Device represents a USB device.
type Device struct {
	name    string // within usbDevicesRoot
	devName string // within /dev
	f       *os.File
}

func newDevice(name string) (*Device, error) {
	dev := &Device{name: name}

	// read DEVNAME= from uevent to locate the device within /dev
	uevent, err := ioutil.ReadFile(dev.sysPath("uevent"))
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(uevent), "\n") {
		if strings.HasPrefix(line, "DEVNAME=") {
			dev.devName = strings.TrimPrefix(line, "DEVNAME=")
		}
	}
	if dev.devName == "" {
		return nil, fmt.Errorf("%q unexpectedly did not not contain a DEVNAME= line", dev.sysPath("uevent"))
	}

	dev.f, err = os.OpenFile(filepath.Join("/dev", dev.devName), os.O_RDWR, 0664)
	if err != nil {
		return nil, err
	}

	// XXX: assumes the scanner always uses interface number 0
	var interfaceNumber uint32
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(dev.f.Fd()), uSBDEVFS_CLAIMINTERFACE, uintptr(unsafe.Pointer(&interfaceNumber))); errno != 0 {
		return nil, errno
	}

	return dev, nil
}

func (u *Device) sysPath(filename string) string {
	return filepath.Join(usbDevicesRoot, u.name, filename)
}

// Read transfers up to len(p) bytes from the device to the host via
// blocking USB bulk transfer.
func (u *Device) Read(p []byte) (n int, err error) {
	bulk := usbdevfsBulkTransfer{
		Ep:      deviceToHost,
		Len:     uint32(len(p)),
		Timeout: uint32((3 * time.Second) / time.Millisecond),
		Data:    &(p[0]),
	}
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(u.f.Fd()), uSBDEVFS_BULK, uintptr(unsafe.Pointer(&bulk))); errno != 0 {
		return 0, errno
	}
	return int(bulk.Len), nil
}

// Write transfers p from the host to the device via blocking USB bulk
// transfer.
func (u *Device) Write(p []byte) (n int, err error) {
	bulk := usbdevfsBulkTransfer{
		Ep:      hostToDevice,
		Len:     uint32(len(p)),
		Timeout: uint32((3 * time.Second) / time.Millisecond),
		Data:    &(p[0]),
	}
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(u.f.Fd()), uSBDEVFS_BULK, uintptr(unsafe.Pointer(&bulk))); errno != 0 {
		return 0, errno
	}
	return len(p), nil
}

// Close releases all resources associated with the Device. The
// Device must not be used after calling Close.
func (u *Device) Close() error {
	// XXX: assumes the scanner always uses interface number 0
	var interfaceNumber uint32
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(u.f.Fd()), uSBDEVFS_RELEASEINTERFACE, uintptr(unsafe.Pointer(&interfaceNumber))); errno != 0 {
		return errno
	}

	return u.f.Close()
}

// badName returns true for names within usbDevicesRoot which do not
// represent a USB device (but a host controller, interface,
// etc.). USB device names consist of digits, dots and dashes,
// starting with a digit.
func badName(name string) bool {
	if name == "" {
		return true
	}

	r, _ := utf8.DecodeRuneInString(name)
	if !unicode.IsDigit(r) {
		return true
	}

	for _, r := range name {
		if r != '.' && r != '-' && !unicode.IsDigit(r) {
			return true
		}
	}

	return false
}

// FindDevice returns a ready-to-use Device object for the Fujitsu
// ScanSnap iX500 or a non-nil error if the scanner is not connected.
func FindDevice() (*Device, error) {
	f, err := os.Open(usbDevicesRoot)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}

	for _, dev := range names {
		if badName(dev) {
			continue
		}
		idProduct, err := ioutil.ReadFile(filepath.Join(usbDevicesRoot, dev, "idProduct"))
		if err != nil {
			return nil, err
		}
		idVendor, err := ioutil.ReadFile(filepath.Join(usbDevicesRoot, dev, "idVendor"))
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(string(idProduct)) == product &&
			strings.TrimSpace(string(idVendor)) == vendor {
			return newDevice(dev)
		}
	}
	return nil, fmt.Errorf("device with product==%q, vendor==%q not found", product, vendor)
}
