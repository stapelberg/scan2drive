package usb

import "errors"

type Device struct{}

func (u *Device) Read(p []byte) (n int, err error) {
	return -1, errors.New("usb access to fss500 is not supported on MacOS")
}

func (u *Device) Write(p []byte) (n int, err error) {
	return -1, errors.New("usb access to fss500 is not supported on MacOS")
}

func (u *Device) Close() error {
	return nil
}

func FindDevice() (*Device, error) {
	return nil, errors.New("usb access to fss500 is not supported on MacOS")
}
