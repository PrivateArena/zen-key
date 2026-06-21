package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"
	"unsafe"
)

func findEventDevice() (string, error) {
	data, err := ioutil.ReadFile("/proc/bus/input/devices")
	if err != nil {
		return "", err
	}

	sections := strings.Split(string(data), "\n\n")
	for _, section := range sections {
		if strings.Contains(section, `Name="ZenFanVirtualKeyboard"`) {
			lines := strings.Split(section, "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "H: Handlers=") {
					parts := strings.Fields(line)
					for _, part := range parts {
						if strings.HasPrefix(part, "event") {
							return "/dev/input/" + part, nil
						}
					}
				}
			}
		}
	}
	return "", fmt.Errorf("ZenFanVirtualKeyboard device not found in /proc/bus/input/devices")
}

func TestInputInjectionKernelLevel(t *testing.T) {
	// 1. Initialize uinput device
	dev, err := InitInputDevice()
	if err != nil {
		t.Fatalf("Failed to open uinput device: %v (Did you run as root / sudo?)", err)
	}
	defer dev.Close()

	// 2. Find the corresponding /dev/input/event* device node
	time.Sleep(500 * time.Millisecond) // Wait for device creation to settle
	devPath, err := findEventDevice()
	if err != nil {
		t.Fatalf("Failed to find event device node: %v", err)
	}
	t.Logf("Found kernel event device node: %s", devPath)

	// 3. Open the event device for reading
	evFile, err := os.Open(devPath)
	if err != nil {
		t.Fatalf("Failed to open event device %s for reading: %v", devPath, err)
	}
	defer evFile.Close()

	// 4. Inject key "A" (keycode 30)
	t.Log("Injecting key 'A' (keycode 30)...")
	dev.InjectKey("A")

	// 5. Read events from the device node and verify reception
	_ = evFile.SetReadDeadline(time.Now().Add(2 * time.Second))
	var ev inputEvent
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&ev)), unsafe.Sizeof(ev))

	pressReceived := false
	releaseReceived := false

	for {
		_, err := evFile.Read(buf)
		if err != nil {
			break // Timeout or EOF
		}

		// evKey (1) corresponds to key events
		if ev.Type == 1 {
			if ev.Code == 30 { // KEY_A
				if ev.Value == 1 {
					t.Log("Successfully read Press event for 'A' from kernel queue")
					pressReceived = true
				} else if ev.Value == 0 {
					t.Log("Successfully read Release event for 'A' from kernel queue")
					releaseReceived = true
				}
			}
		}

		if pressReceived && releaseReceived {
			break
		}
	}

	if !pressReceived || !releaseReceived {
		t.Fatalf("FAILED: Did not receive both Press and Release kernel events for injected key (Press=%t, Release=%t)", pressReceived, releaseReceived)
	}

	t.Log("SUCCESS: Keystroke successfully round-tripped through Linux input subsystem!")
}
