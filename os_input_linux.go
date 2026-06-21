package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// Minimal explicit uinput implementation to bypass heavy external dependencies
const (
	uinputMaxNameSize = 80
	evKey             = 0x01
	evSyn             = 0x00
	synReport         = 0

	// Modern verified ioctl numbers for x86_64 Linux
	uiSetEvbit  = 1074025828
	uiSetKeybit = 1074025829
	uiDevSetup  = 1079792899
	uiDevCreate = 21761
)

type inputID struct {
	Bustype uint16
	Vendor  uint16
	Product uint16
	Version uint16
}

// uinputSetup matches the C struct uinput_setup in <linux/uinput.h>
type uinputSetup struct {
	ID           inputID
	Name         [uinputMaxNameSize]byte
	FFEffectsMax uint32
}

type inputEvent struct {
	Time  syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

type InputDevice struct {
	file *os.File
}

// Map characters to standard Linux input codes
var charToKeyCode = map[string]uint16{
	"A": 30, "B": 48, "C": 46, "D": 32, "E": 18, "F": 33, "G": 34, "H": 35,
	"I": 23, "J": 36, "K": 37, "L": 38, "M": 50, "N": 49, "O": 24, "P": 25,
	"Q": 16, "R": 19, "S": 31, "T": 20, "U": 22, "V": 47, "W": 17, "X": 45,
	"Y": 21, "Z": 44,
}

func InitInputDevice() (*InputDevice, error) {
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/uinput: %v (Check permissions)", err)
	}

	// 1. Enable Key Events
	if err := ioctl(f.Fd(), uiSetEvbit, evKey); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to set EVBIT: %v", err)
	}

	// 2. Enable specific keycodes
	for _, code := range charToKeyCode {
		if err := ioctl(f.Fd(), uiSetKeybit, uintptr(code)); err != nil {
			f.Close()
			return nil, fmt.Errorf("failed to set KEYBIT for code %d: %v", code, err)
		}
	}

	// 3. Initialize the setup structure
	var setup uinputSetup
	setup.ID = inputID{
		Bustype: 0x03, // BUS_USB
		Vendor:  0x1234,
		Product: 0x5678,
		Version: 1,
	}
	copy(setup.Name[:], "ZenFanVirtualKeyboard")

	// 4. Issue UI_DEV_SETUP ioctl
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		uiDevSetup,
		uintptr(unsafe.Pointer(&setup)),
	)
	if errno != 0 {
		f.Close()
		return nil, fmt.Errorf("UI_DEV_SETUP ioctl failed: %v", errno)
	}

	// 5. Create the virtual device
	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uiDevCreate, 0)
	if errno != 0 {
		f.Close()
		return nil, fmt.Errorf("UI_DEV_CREATE ioctl failed: %v", errno)
	}

	return &InputDevice{file: f}, nil
}

func (id *InputDevice) InjectKey(char string) {
	code, exists := charToKeyCode[char]
	if !exists {
		return
	}
	// Press
	id.writeEvent(evKey, code, 1)
	id.writeEvent(evSyn, synReport, 0)
	// Release
	id.writeEvent(evKey, code, 0)
	id.writeEvent(evSyn, synReport, 0)
}

func (id *InputDevice) Close() {
	if id.file != nil {
		id.file.Close()
	}
}

// System call helper for standard integer ioctls
func ioctl(fd uintptr, request uintptr, val uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, val)
	if errno != 0 {
		return errno
	}
	return nil
}

// Helper to write input_event to uinput device
func (id *InputDevice) writeEvent(evType, code uint16, value int32) {
	var ev inputEvent
	ev.Type = evType
	ev.Code = code
	ev.Value = value
	
	// Write the struct using safe slice cast and File.Write
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&ev)), unsafe.Sizeof(ev))
	_, _ = id.file.Write(buf)
}
