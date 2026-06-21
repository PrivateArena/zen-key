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
	"gioui.org/widget/material"
)

type AppUI struct {
	Config         Config
	Keys           []KeyDefinition
	IsExpanded     bool
	HoveredKey     string
	InputDevice    *InputDevice
	Window         *app.Window
	TargetWindowID string
	Theme          *material.Theme
}

func NewAppUI(cfg Config, inputDev *InputDevice, w *app.Window, th *material.Theme) *AppUI {
	expW := float32(cfg.GetExpandedWidth())
	expH := float32(cfg.GetExpandedHeight())
	return &AppUI{
		Config:      cfg,
		Keys:        GetLayout(cfg.LayoutStyle).Arrange(cfg, expW, expH, 1.0),
		IsExpanded:  false,
		InputDevice: inputDev,
		Window:      w,
		Theme:       th,
	}
}

func (ui *AppUI) Layout(gtx layout.Context) layout.Dimensions {
	// Dynamically arrange keys scaled to active device DPI / Scale Factor
	w := float32(gtx.Constraints.Max.X)
	h := float32(gtx.Constraints.Max.Y)
	ui.Keys = GetLayout(ui.Config.LayoutStyle).Arrange(ui.Config, w, h, gtx.Metric.PxPerDp)

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
						go RepositionWindow(ui.Config.ScreenPosition, 60, 60)
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

func (ui *AppUI) GetPivot(gtx layout.Context) (float32, float32) {
	w := float32(gtx.Constraints.Max.X)
	h := float32(gtx.Constraints.Max.Y)
	scale := gtx.Metric.PxPerDp

	padLeft := ui.Config.Padding.Left * scale
	padRight := ui.Config.Padding.Right * scale
	padTop := ui.Config.Padding.Top * scale
	padBottom := ui.Config.Padding.Bottom * scale

	switch ui.Config.ScreenPosition {
	case "top-left":
		return padLeft, padTop
	case "top-right":
		return w - padRight, padTop
	case "bottom-left":
		return padLeft, h - padBottom
	default: // "bottom-right"
		return w - padRight, h - padBottom
	}
}

func (ui *AppUI) ProcessMouseMove(gtx layout.Context, pos f32.Point) {
	w := float32(gtx.Constraints.Max.X)
	h := float32(gtx.Constraints.Max.Y)
	scale := gtx.Metric.PxPerDp

	pivotX, pivotY := ui.GetPivot(gtx)
	dx := pos.X - pivotX
	dy := pos.Y - pivotY
	distance := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if !ui.IsExpanded {
		// Detect touch tracking intersection inside floating trigger button (center of collapsed window)
		cx := w / 2
		cy := h / 2
		dx := pos.X - cx
		dy := pos.Y - cy
		dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if dist <= (20.0 * scale) {
			// Find active window ID BEFORE expanding (to restore focus when typing)
			activeWinOut, err := exec.Command("xdotool", "getactivewindow").Output()
			if err == nil {
				ui.TargetWindowID = strings.TrimSpace(string(activeWinOut))
			}
			
			ui.IsExpanded = true
			if ui.Window != nil {
				expW := ui.Config.GetExpandedWidth()
				expH := ui.Config.GetExpandedHeight()
				ui.Window.Option(app.Size(unit.Dp(expW), unit.Dp(expH)))
				go RepositionWindow(ui.Config.ScreenPosition, expW, expH)
			}
			gtx.Execute(op.InvalidateCmd{}) // Immediate window loop state redraw
		}
	} else {
		// Collapse back cleanly if cursor escapes outer activation boundaries of the keys
		var maxKeyDist float32
		for _, key := range ui.Keys {
			dist := float32(math.Sqrt(float64(key.X*key.X + key.Y*key.Y)))
			if dist > maxKeyDist {
				maxKeyDist = dist
			}
		}
		if distance > maxKeyDist+(40.0*scale) {
			ui.IsExpanded = false
			ui.HoveredKey = ""
			if ui.Window != nil {
				ui.Window.Option(app.Size(unit.Dp(60), unit.Dp(60)))
				go RepositionWindow(ui.Config.ScreenPosition, 60, 60)
			}
			gtx.Execute(op.InvalidateCmd{})
			return
		}

		// Calculate matching keys using precise spatial targets based on scaled key sizes
		ui.HoveredKey = ""
		for _, key := range ui.Keys {
			kx := pivotX + key.X
			ky := pivotY + key.Y
			kdx := pos.X - kx
			kdy := pos.Y - ky
			hitRadius := key.KeySize / 2.0
			if key.KeySize == 0 {
				hitRadius = (ui.Config.KeySize / 2.0) * ui.Config.KeyboardScale * scale
			}
			if math.Sqrt(float64(kdx*kdx+kdy*kdy)) < float64(hitRadius) {
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
	scale := gtx.Metric.PxPerDp

	if !ui.IsExpanded {
		// Draw a beautiful glassmorphism-style floating trigger button
		cx := w / 2
		cy := h / 2
		r := float32(20.0 * scale)
		
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
			Min: image.Pt(int(cx-float32(6*scale)), int(cy-float32(6*scale))),
			Max: image.Pt(int(cx+float32(6*scale)), int(cy+float32(6*scale))),
		}.Op(gtx.Ops).Push(gtx.Ops)
		paint.ColorOp{Color: color.NRGBA{R: 130, G: 110, B: 230, A: 200}}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		coreStack.Pop()
		return
	}

	// Render Base Background Layer based on active layout
	pivotX, pivotY := ui.GetPivot(gtx)

	if ui.Config.LayoutStyle == "grid" {
		layoutImpl := &GridLayout{}
		layoutImpl.RenderBackground(gtx, ui.Config, pivotX, pivotY, scale)
	} else {
		layoutImpl := &FanLayout{}
		layoutImpl.RenderBackground(gtx, ui.Config, pivotX, pivotY, scale)
	}

	// Draw Individual Keys
	for _, key := range ui.Keys {
		kx := pivotX + key.X
		ky := pivotY + key.Y

		circleRadius := key.KeySize / 2.0
		if key.KeySize == 0 {
			circleRadius = (ui.Config.KeySize / 2.0) * ui.Config.KeyboardScale * scale
		}
		keyColor := color.NRGBA{R: 50, G: 50, B: 54, A: 255}
		if key.Char == ui.HoveredKey {
			circleRadius = circleRadius + 4.0*scale // Micro-interaction expand feedback
			keyColor = color.NRGBA{R: 140, G: 140, B: 145, A: 255}
		}

		keyShape := clip.Ellipse{
			Min: image.Pt(int(kx-circleRadius), int(ky-circleRadius)),
			Max: image.Pt(int(kx+circleRadius), int(ky+circleRadius)),
		}.Op(gtx.Ops)
		paint.FillShape(gtx.Ops, keyColor, keyShape)

		// Draw the key label character centered inside the circle
		radDp := circleRadius / scale
		if key.Char == ui.HoveredKey {
			radDp = radDp - 4.0 // Normalize hover expansion for font scaling consistency
		}
		fontSize := unit.Sp(12 * radDp / 18.0)
		if fontSize < 8 {
			fontSize = 8
		}

		// Center the text based on character length and keycaps size
		charLen := float32(len(key.Char))
		offX := charLen * 3.5 * (radDp / 18.0) * scale
		offY := 6.0 * (radDp / 18.0) * scale
		labelOffset := op.Offset(image.Pt(int(kx-offX), int(ky-offY))).Push(gtx.Ops)
		label := material.Label(ui.Theme, fontSize, key.Char)
		label.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		
		// Temporarily expand constraints for layout so it is not clipped/wrapped
		labelGtx := gtx
		labelGtx.Constraints = layout.Exact(image.Point{X: int(100 * scale), Y: int(100 * scale)})
		label.Layout(labelGtx)
		labelOffset.Pop()
	}
}

func (g *GridLayout) RenderBackground(gtx layout.Context, cfg Config, pivotX, pivotY float32, scale float32) {
	totalScale := cfg.KeyboardScale * scale
	keySize := cfg.KeySize * totalScale
	gap := cfg.KeyGap * totalScale

	cols := 6
	rows := 3

	cellSize := keySize + gap
	w := float32(cols) * cellSize
	h := float32(rows) * cellSize

	var minX, minY, maxX, maxY float32
	switch cfg.ScreenPosition {
	case "top-left":
		minX = pivotX - gap
		minY = pivotY - gap
		maxX = pivotX + w
		maxY = pivotY + h
	case "top-right":
		minX = pivotX - w
		minY = pivotY - gap
		maxX = pivotX + gap
		maxY = pivotY + h
	case "bottom-left":
		minX = pivotX - gap
		minY = pivotY - h
		maxX = pivotX + w
		maxY = pivotY + gap
	default: // "bottom-right"
		minX = pivotX - w
		minY = pivotY - h
		maxX = pivotX + gap
		maxY = pivotY + gap
	}

	bgShape := clip.RRect{
		Rect: image.Rectangle{
			Min: image.Point{X: int(minX), Y: int(minY)},
			Max: image.Point{X: int(maxX), Y: int(maxY)},
		},
		SE:   int(12 * scale),
		SW:   int(12 * scale),
		NW:   int(12 * scale),
		NE:   int(12 * scale),
	}.Op(gtx.Ops)
	paint.FillShape(gtx.Ops, color.NRGBA{R: 28, G: 28, B: 30, A: 235}, bgShape)
}

func (f *FanLayout) RenderBackground(gtx layout.Context, cfg Config, pivotX, pivotY float32, scale float32) {
	totalScale := cfg.KeyboardScale * scale
	outerRadScaled := cfg.FanConfig.OuterRadius * totalScale
	
	startAngle, endAngle := cfg.FanConfig.StartAngle, cfg.FanConfig.EndAngle
	if startAngle == 0 && endAngle == 0 {
		startAngle, endAngle = GetDefaultAngles(cfg.ScreenPosition)
	}

	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(pivotX, pivotY))
	
	startRad := float64(startAngle * math.Pi / 180.0)
	endRad := float64(endAngle * math.Pi / 180.0)
	
	sx := pivotX + outerRadScaled*float32(math.Cos(startRad))
	sy := pivotY + outerRadScaled*float32(math.Sin(startRad))
	path.LineTo(f32.Pt(sx, sy))
	
	steps := 16
	for i := 1; i <= steps; i++ {
		pct := float64(i) / float64(steps)
		currAngle := startRad + pct*(endRad-startRad)
		ax := pivotX + outerRadScaled*float32(math.Cos(currAngle))
		ay := pivotY + outerRadScaled*float32(math.Sin(currAngle))
		path.LineTo(f32.Pt(ax, ay))
	}
	path.LineTo(f32.Pt(pivotX, pivotY))
	path.Close()

	bgShape := clip.Outline{Path: path.End()}.Op()
	paint.FillShape(gtx.Ops, color.NRGBA{R: 28, G: 28, B: 30, A: 235}, bgShape)
}

func RepositionWindow(posType string, w, h int) {
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

	var x, y int
	switch posType {
	case "top-left":
		x = 0
		y = 0
	case "top-right":
		x = scrW - w
		y = 0
	case "bottom-left":
		x = 0
		y = scrH - h
	default: // "bottom-right"
		x = scrW - w
		y = scrH - h
	}

	_ = exec.Command("xdotool", "windowmove", winID, strconv.Itoa(x), strconv.Itoa(y)).Run()
	_ = exec.Command("xprop", "-id", winID, "-f", "_NET_WM_WINDOW_TYPE", "32a", "-set", "_NET_WM_WINDOW_TYPE", "_NET_WM_WINDOW_TYPE_POPUP_MENU").Run()
	_ = exec.Command("xprop", "-id", winID, "-format", "WM_HINTS", "32cbcxxiixx", "-set", "WM_HINTS", "3,False,1,0x0,0x0,0,0,0x0,0x0").Run()
	_ = exec.Command("xprop", "-id", winID, "-remove", "WM_TAKE_FOCUS").Run()
	_ = exec.Command("wmctrl", "-i", "-r", winID, "-b", "add,above").Run()
}

