package gpio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"syscall"
	"time"
	"unsafe"
)

const (
	GPIOHANDLE_REQUEST_INPUT             = (1 << 0)
	GPIOHANDLE_REQUEST_OUTPUT            = (1 << 1)
	GPIOHANDLE_REQUEST_ACTIVE_LOW        = (1 << 2)
	GPIOHANDLE_REQUEST_OPEN_DRAIN        = (1 << 3)
	GPIOHANDLE_REQUEST_OPEN_SOURCE       = (1 << 4)
	GPIOHANDLE_REQUEST_BIAS_PULL_UP      = (1 << 5)
	GPIOHANDLE_REQUEST_BIAS_PULL_DOWN    = (1 << 6)
	GPIOHANDLE_REQUEST_BIAS_PULL_DISABLE = (1 << 7)
)

const (
	GPIO_GET_LINEHANDLE_IOCTL = 0xc16cb403
	GPIO_GET_LINEEVENT_IOCTL  = 0xc030b404
)

const (
	GPIOHANDLE_SET_LINE_VALUES_IOCTL = 0xc040b409
	GPIOHANDLE_GET_LINE_VALUES_IOCTL = 0xc040b408
)

const (
	GPIOEVENT_REQUEST_RISING_EDGE  = (1 << 0)
	GPIOEVENT_REQUEST_FALLING_EDGE = (1 << 1)
	GPIOEVENT_REQUEST_BOTH_EDGES   = GPIOEVENT_REQUEST_RISING_EDGE |
		GPIOEVENT_REQUEST_FALLING_EDGE
)

type gpiohandlerequest struct {
	Lineoffsets   [64]uint32
	Flags         uint32
	DefaultValues [64]uint8
	ConsumerLabel [32]byte
	Lines         uint32
	Fd            uintptr
}

type gpioeventrequest struct {
	LineOffset    uint32
	HandleFlags   uint32
	EventFlags    uint32
	ConsumerLabel [32]byte
	Fd            uint32
}

type gpioeventdata struct {
	Timestamp uint64
	Id        uint32
	_         uint32 // padding to match C compiler
}

type gpiohandledata struct {
	Values [64]uint8
}

type GPIO struct {
	f *os.File
}

func NewGPIO() (*GPIO, error) {
	f, err := os.Open("/dev/gpiochip0")
	if err != nil {
		return nil, err
	}

	return &GPIO{
		f: f,
	}, nil
}

func (g *GPIO) Close() error {
	return g.f.Close()
}

type Keypress struct {
}

func (g *GPIO) NotifyKeypresses(lineOffset uint32, ch chan<- Keypress) error {
	eventreq := gpioeventrequest{
		LineOffset:    lineOffset,
		HandleFlags:   GPIOHANDLE_REQUEST_BIAS_PULL_UP,
		EventFlags:    GPIOEVENT_REQUEST_FALLING_EDGE,
		ConsumerLabel: [32]byte{'s', 'c', 'a', 'n', '2', 'd', 'r', 'i', 'v', 'e'},
		Fd:            7,
	}

	// We cannot use unsafe.Pointer(&eventreq) directly because the Go compiler
	// and the C compiler differ in their struct alignment. This results in Go
	// accessing the Fd field in a different spot as the Linux kernel.
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, eventreq); err != nil {
		return err
	}
	b := buf.Bytes()
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(g.f.Fd()), GPIO_GET_LINEEVENT_IOCTL, uintptr(unsafe.Pointer(&b[0]))); errno != 0 {
		return fmt.Errorf("GET_LINEEVENT: %v", errno)
	}
	fd := binary.LittleEndian.Uint32(b[44:])

	f := os.NewFile(uintptr(fd), "eventreq.Fd")

	get := func(fd uintptr) (byte, error) {
		idata := gpiohandledata{
			Values: [64]uint8{},
		}

		if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), GPIOHANDLE_GET_LINE_VALUES_IOCTL, uintptr(unsafe.Pointer(&idata))); errno != 0 {
			return 0, errno
		}
		return idata.Values[0], nil
	}

	const (
		High = 1
		Low  = 0
	)

	go func() {
		defer close(ch)
		for {
			// Blockingly wait for a line event:
			var data gpioeventdata
			if err := binary.Read(f, binary.LittleEndian, &data); err != nil {
				log.Println(err)
				return
			}
			// Verify state is Low. While the key bounces, this might return High
			// despite the line event only triggering on the falling edge.
			v, err := get(uintptr(fd))
			if err != nil {
				log.Println(err)
				return
			}
			if v != Low {
				continue
			}

			ch <- Keypress{}

			// Cheap debounce: wait until the key is High again for 1 second.  We
			// can do this here (but not in a keyboard controller) because the same
			// key does not need to be pressed multiple times in quick succession.
			start := time.Now()
			for time.Since(start) < 1*time.Second {
				newv, err := get(uintptr(fd))
				if err != nil {
					log.Println(err)
					return
				}
				if newv == Low {
					// reset timer
					start = time.Now()
				}
				time.Sleep(250 * time.Millisecond)
			}
		}
	}()
	return nil
}
