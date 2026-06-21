package main

import (
	"image/color"
	"log"
	"os"
	"runtime"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

func main() {
	// Restrict UI tracking thread strictly to active OS context mapping standard
	runtime.LockOSThread()

	inputDev, err := InitInputDevice()
	if err != nil {
		log.Printf("Warning: Input injection layer initialization bypassed: %v", err)
	} else {
		defer inputDev.Close()
	}

	go func() {
		// Initialize borderless fullscreen window context layout
		w := new(app.Window)
		w.Option(app.Title("Zen Fan Keyboard"))
		w.Option(app.Decorated(false))      // Remove frame borders entirely
		w.Option(app.TopMost(true))         // Ensure it sits above taskbars/docks
		w.Option(app.Size(unit.Dp(60), unit.Dp(60))) // Start collapsed
		
		// Set Gio initialization configurations
		ui := NewAppUI(DefaultConfig, inputDev, w)
		if err := runWindow(w, ui); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()

	app.Main()
}

func runWindow(w *app.Window, ui *AppUI) error {
	var ops op.Ops
	initialized := false
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			
			// Position window at bottom-right on startup
			if !initialized {
				initialized = true
				go RepositionWindow(60, 60)
			}

			// Clear the frame with dark theme background (fallback for lack of transparency support)
			paint.Fill(gtx.Ops, color.NRGBA{R: 28, G: 28, B: 30, A: 255})

			// Force layout metric units scale parameters mapping
			gtx.Constraints.Max = e.Size
			ui.Layout(gtx)
			
			e.Frame(gtx.Ops)
		}
	}
}
