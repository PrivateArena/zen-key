package main

import (
	"image"
	"image/color"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

type AppUI struct {
	Config         Config
	Keys           []KeyDefinition
	IsExpanded     bool
	HoveredKey     string
	InputDevice    *InputDevice
	Window         *app.Window
	TargetWindowID string
}

func NewAppUI(cfg Config, inputDev *InputDevice, w *app.Window) *AppUI {
	return &AppUI{
		Config:      cfg,
		Keys:        CalculateFanKeys(cfg),
		IsExpanded:  false,
		InputDevice: inputDev,
		Window:      w,
	}
}

func (ui *AppUI) Layout(gtx layout.Context) layout.Dimensions {
	// Handle continuous reactive pointer events
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: ui,
			Kinds:  pointer.Move | pointer.Press | pointer.Leave,
		})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			switch e.Kind {
			case pointer.Move:
				ui.ProcessMouseMove(gtx, e.Position)
			case pointer.Leave:
				// Collapse immediately if mouse leaves the window bounds
				if ui.IsExpanded {
					ui.IsExpanded = false
					ui.HoveredKey = ""
					if ui.Window != nil {
						ui.Window.Option(app.Size(unit.Dp(60), unit.Dp(60)))
						go RepositionWindow(60, 60)
					}
					gtx.Execute(op.InvalidateCmd{})
				}
			case pointer.Press:
				if ui.IsExpanded && ui.HoveredKey != "" {
					if ui.TargetWindowID != "" {
						_ = exec.Command("xdotool", "windowactivate", ui.TargetWindowID).Run()
						time.Sleep(50 * time.Millisecond) // Let window manager focus settle
					}
					if ui.InputDevice != nil {
						ui.InputDevice.InjectKey(ui.HoveredKey)
					}
				}
			}
		}
	}

	// Constrain pointer mapping scope across window frame
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, ui)
	area.Pop()

	ui.Render(gtx)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (ui *AppUI) ProcessMouseMove(gtx layout.Context, pos f32.Point) {
	w := float32(gtx.Constraints.Max.X)
	h := float32(gtx.Constraints.Max.Y)

	// Distance tracking from the absolute bottom-right corner pivot point
	dx := pos.X - w
	dy := pos.Y - h
	distance := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if !ui.IsExpanded {
		// Detect touch tracking intersection inside floating circle trigger button
		cx := w - 30
		cy := h - 30
		dx := pos.X - cx
		dy := pos.Y - cy
		dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if dist <= 20.0 {
			// Find active window ID BEFORE expanding (to restore focus when typing)
			activeWinOut, err := exec.Command("xdotool", "getactivewindow").Output()
			if err == nil {
				ui.TargetWindowID = strings.TrimSpace(string(activeWinOut))
			}
			
			ui.IsExpanded = true
			if ui.Window != nil {
				ui.Window.Option(app.Size(unit.Dp(260), unit.Dp(260)))
				go RepositionWindow(260, 260)
			}
			gtx.Execute(op.InvalidateCmd{}) // Immediate window loop state redraw
		}
	} else {
		// Collapses back cleanly if cursor escapes outer activation boundaries
		if distance > ui.Config.OuterRadius+40.0 {
			ui.IsExpanded = false
			ui.HoveredKey = ""
			if ui.Window != nil {
				ui.Window.Option(app.Size(unit.Dp(60), unit.Dp(60)))
				go RepositionWindow(60, 60)
			}
			gtx.Execute(op.InvalidateCmd{})
			return
		}

		// Calculate matching keys using precise spatial targets
		ui.HoveredKey = ""
		for _, key := range ui.Keys {
			kx := w + key.X
			ky := h + key.Y
			kdx := pos.X - kx
			kdy := pos.Y - ky
			if math.Sqrt(float64(kdx*kdx+kdy*kdy)) < 22.0 { // 22px target precision bounds
				ui.HoveredKey = key.Char
				break
			}
		}
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *AppUI) Render(gtx layout.Context) {
	w := float32(gtx.Constraints.Max.X)
	h := float32(gtx.Constraints.Max.Y)

	if !ui.IsExpanded {
		// Draw a beautiful glassmorphism-style floating trigger button
		cx := w - 30
		cy := h - 30
		r := float32(20.0)
		
		stack := clip.Ellipse{
			Min: image.Pt(int(cx-r), int(cy-r)),
			Max: image.Pt(int(cx+r), int(cy+r)),
		}.Op(gtx.Ops).Push(gtx.Ops)
		
		// Glassmorphic translucent indicator
		paint.ColorOp{Color: color.NRGBA{R: 255, G: 255, B: 255, A: 60}}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		stack.Pop()
		
		// Subtle purple-ish active core dot
		coreStack := clip.Ellipse{
			Min: image.Pt(int(cx-6), int(cy-6)),
			Max: image.Pt(int(cx+6), int(cy+6)),
		}.Op(gtx.Ops).Push(gtx.Ops)
		paint.ColorOp{Color: color.NRGBA{R: 130, G: 110, B: 230, A: 200}}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		coreStack.Pop()
		return
	}

	// Render Fan Base Background Layer (Dark Transparent Zen Panel Segment)
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(w, h))
	
	startRad := float64(ui.Config.StartAngle * math.Pi / 180.0)
	endRad := float64(ui.Config.EndAngle * math.Pi / 180.0)
	
	sx := w + ui.Config.OuterRadius*float32(math.Cos(startRad))
	sy := h + ui.Config.OuterRadius*float32(math.Sin(startRad))
	path.LineTo(f32.Pt(sx, sy))
	
	// Create smooth arc using Bezier interpolation increments
	steps := 16
	for i := 1; i <= steps; i++ {
		pct := float64(i) / float64(steps)
		currAngle := startRad + pct*(endRad-startRad)
		ax := w + ui.Config.OuterRadius*float32(math.Cos(currAngle))
		ay := h + ui.Config.OuterRadius*float32(math.Sin(currAngle))
		path.LineTo(f32.Pt(ax, ay))
	}
	path.LineTo(f32.Pt(w, h))
	path.Close()

	bgShape := clip.Outline{Path: path.End()}.Op()
	paint.FillShape(gtx.Ops, color.NRGBA{R: 28, G: 28, B: 30, A: 235}, bgShape)

	// Draw Individual Keys Along Radial Segment Path Arrays
	for _, key := range ui.Keys {
		kx := w + key.X
		ky := h + key.Y

		circleRadius := float32(18.0)
		keyColor := color.NRGBA{R: 50, G: 50, B: 54, A: 255}
		if key.Char == ui.HoveredKey {
			circleRadius = 22.0 // Micro-interaction expand scale feedback effect
			keyColor = color.NRGBA{R: 140, G: 140, B: 145, A: 255}
		}

		keyShape := clip.Ellipse{
			Min: image.Pt(int(kx-circleRadius), int(ky-circleRadius)),
			Max: image.Pt(int(kx+circleRadius), int(ky+circleRadius)),
		}.Op(gtx.Ops)
		paint.FillShape(gtx.Ops, keyColor, keyShape)
	}
}

func RepositionWindow(w, h int) {
	time.Sleep(50 * time.Millisecond)

	out, err := exec.Command("xdotool", "search", "--name", "Zen Fan Keyboard").Output()
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return
	}
	winID := lines[len(lines)-1]

	geomOut, err := exec.Command("xdotool", "getdisplaygeometry").Output()
	if err != nil {
		return
	}
	parts := strings.Fields(string(geomOut))
	if len(parts) < 2 {
		return
	}
	scrW, _ := strconv.Atoi(parts[0])
	scrH, _ := strconv.Atoi(parts[1])

	// Calculate bottom-right position
	x := scrW - w
	y := scrH - h

	_ = exec.Command("xdotool", "windowmove", winID, strconv.Itoa(x), strconv.Itoa(y)).Run()
	_ = exec.Command("xprop", "-id", winID, "-f", "_NET_WM_WINDOW_TYPE", "32a", "-set", "_NET_WM_WINDOW_TYPE", "_NET_WM_WINDOW_TYPE_UTILITY").Run()
	_ = exec.Command("xprop", "-id", winID, "-format", "WM_HINTS", "32cbcxxiixx", "-set", "WM_HINTS", "3,False,1,0x0,0x0,0,0,0x0,0x0").Run()
	_ = exec.Command("xprop", "-id", winID, "-remove", "WM_TAKE_FOCUS").Run()
	_ = exec.Command("wmctrl", "-i", "-r", winID, "-b", "add,above").Run()
}
