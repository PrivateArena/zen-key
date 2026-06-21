package main

import (
	"encoding/json"
	"image"
	"math"
	"os"
)

type Padding struct {
	Top    float32 `json:"top"`
	Right  float32 `json:"right"`
	Bottom float32 `json:"bottom"`
	Left   float32 `json:"left"`
}

type FanConfig struct {
	InnerRadius float32 `json:"inner_radius"`
	OuterRadius float32 `json:"outer_radius"`
	StartAngle  float32 `json:"start_angle"`
	EndAngle    float32 `json:"end_angle"`
}

type Config struct {
	LayoutStyle    string      `json:"layout_style"`
	KeyboardScale  float32     `json:"keyboard_scale"`
	KeySize        float32     `json:"key_size"`
	KeyGap         float32     `json:"key_gap"`
	ScreenPosition string      `json:"screen_position"`
	Padding        Padding     `json:"padding"`
	FanConfig      FanConfig   `json:"fan_config"`
	KeyRings       [][]string  `json:"key_rings"`
	ExpandedWidth  int         `json:"expanded_width"`
	ExpandedHeight int         `json:"expanded_height"`
}

type KeyDefinition struct {
	Char     string
	X, Y     float32 // Coordinates relative to pivot
	Radius   float32
	AngleDeg float32
	KeySize  float32 // Dynamic visual size of the key
}

var DefaultKeyRings = [][]string{
	{"E", "T", "A", "O", "I", "N"}, // Inner Ring
	{"S", "H", "R", "D", "L", "C"}, // Middle Ring
	{"U", "M", "W", "F", "G", "Y"}, // Outer Ring
}

var DefaultConfig = Config{
	LayoutStyle:    "fan",
	KeyboardScale:  1.0,
	KeySize:        36.0,
	KeyGap:         8.0,
	ScreenPosition: "bottom-right",
	Padding: Padding{
		Top:    45.0,
		Right:  45.0,
		Bottom: 45.0,
		Left:   45.0,
	},
	FanConfig: FanConfig{
		InnerRadius: 70.0,
		OuterRadius: 230.0,
		StartAngle:  0, // 0 means use default based on ScreenPosition
		EndAngle:    0,
	},
	KeyRings: DefaultKeyRings,
}

func LoadConfig() (*Config, error) {
	file, err := os.Open("config.json")
	if err != nil {
		if os.IsNotExist(err) {
			data, err := json.MarshalIndent(DefaultConfig, "", "  ")
			if err == nil {
				_ = os.WriteFile("config.json", data, 0644)
			}
			return &DefaultConfig, nil
		}
		return nil, err
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}
	if len(cfg.KeyRings) == 0 {
		cfg.KeyRings = DefaultKeyRings
	}
	return &cfg, nil
}

// GetDefaultAngles returns the quadrant angle range based on screen position
func GetDefaultAngles(pos string) (float32, float32) {
	switch pos {
	case "top-left":
		return 0, 90
	case "top-right":
		return 90, 180
	case "bottom-left":
		return 270, 360
	default: // "bottom-right"
		return 180, 270
	}
}

// KeyboardLayout defines the interface for different keyboard geometry layout styles
type KeyboardLayout interface {
	Arrange(cfg Config, winW, winH float32, scale float32) []KeyDefinition
}

// FanLayout places keys along concentric arc rings radiating from a corner pivot
type FanLayout struct{}

func (f *FanLayout) Arrange(cfg Config, winW, winH float32, scale float32) []KeyDefinition {
	var keys []KeyDefinition
	totalRings := len(cfg.KeyRings)
	if totalRings == 0 {
		return keys
	}

	padLeft := cfg.Padding.Left * scale
	padRight := cfg.Padding.Right * scale
	padTop := cfg.Padding.Top * scale
	padBottom := cfg.Padding.Bottom * scale

	var availW, availH float32
	switch cfg.ScreenPosition {
	case "top-left":
		availW = winW - padLeft
		availH = winH - padTop
	case "top-right":
		availW = winW - padRight
		availH = winH - padTop
	case "bottom-left":
		availW = winW - padLeft
		availH = winH - padBottom
	default: // "bottom-right"
		availW = winW - padRight
		availH = winH - padBottom
	}

	maxRadius := availW
	if availH < maxRadius {
		maxRadius = availH
	}
	keySize := cfg.KeySize * cfg.KeyboardScale * scale
	maxRadius = maxRadius - (keySize / 2.0) - (cfg.KeyGap * scale)

	innerRad := cfg.FanConfig.InnerRadius * cfg.KeyboardScale * scale
	if innerRad == 0 {
		innerRad = maxRadius * 0.20
	}
	outerRad := cfg.FanConfig.OuterRadius * cfg.KeyboardScale * scale
	if outerRad == 0 || outerRad > maxRadius {
		outerRad = maxRadius
	}
	if outerRad < innerRad {
		outerRad = innerRad + (cfg.KeySize+cfg.KeyGap)*cfg.KeyboardScale*scale*float32(totalRings-1)
		if outerRad > maxRadius {
			outerRad = maxRadius
		}
	}

	startAngle, endAngle := cfg.FanConfig.StartAngle, cfg.FanConfig.EndAngle
	if startAngle == 0 && endAngle == 0 {
		startAngle, endAngle = GetDefaultAngles(cfg.ScreenPosition)
	}
	sweepAngle := endAngle - startAngle

	for ringIdx, ring := range cfg.KeyRings {
		var radius float32
		if totalRings > 1 {
			radius = innerRad + (float32(ringIdx)/float32(totalRings-1))*(outerRad-innerRad)
		} else {
			radius = innerRad
		}

		keysInRing := len(ring)
		for charIdx, char := range ring {
			var angleDeg float32
			if keysInRing > 1 {
				angleDeg = startAngle + (float32(charIdx)/float32(keysInRing-1))*sweepAngle
			} else {
				angleDeg = startAngle + sweepAngle/2
			}

			angleRad := float64(angleDeg * math.Pi / 180.0)

			// Polar to Cartesian coordinate mapping relative to pivot
			x := radius * float32(math.Cos(angleRad))
			y := radius * float32(math.Sin(angleRad))

			keys = append(keys, KeyDefinition{
				Char:     char,
				X:        x,
				Y:        y,
				Radius:   radius,
				AngleDeg: angleDeg,
				KeySize:  keySize,
			})
		}
	}
	return keys
}

// GridLayout places keys in rows and columns radiating from a corner pivot
type GridLayout struct{}

func (g *GridLayout) Arrange(cfg Config, winW, winH float32, scale float32) []KeyDefinition {
	var keys []KeyDefinition
	totalRings := len(cfg.KeyRings)
	if totalRings == 0 {
		return keys
	}

	padLeft := cfg.Padding.Left * scale
	padRight := cfg.Padding.Right * scale
	padTop := cfg.Padding.Top * scale
	padBottom := cfg.Padding.Bottom * scale

	var availW, availH float32
	switch cfg.ScreenPosition {
	case "top-left":
		availW = winW - padLeft
		availH = winH - padTop
	case "top-right":
		availW = winW - padRight
		availH = winH - padTop
	case "bottom-left":
		availW = winW - padLeft
		availH = winH - padBottom
	default: // "bottom-right"
		availW = winW - padRight
		availH = winH - padBottom
	}

	maxCols := 0
	for _, ring := range cfg.KeyRings {
		if len(ring) > maxCols {
			maxCols = len(ring)
		}
	}
	if maxCols == 0 {
		return keys
	}

	cellW := availW / float32(maxCols)
	cellH := availH / float32(totalRings)

	cellSize := cellW
	if cellH < cellSize {
		cellSize = cellH
	}

	gap := cfg.KeyGap * scale
	keySize := cellSize - gap
	if keySize < 20*scale {
		keySize = 20 * scale
	}

	var dirX, dirY float32
	switch cfg.ScreenPosition {
	case "top-left":
		dirX, dirY = 1, 1
	case "top-right":
		dirX, dirY = -1, 1
	case "bottom-left":
		dirX, dirY = 1, -1
	default: // "bottom-right"
		dirX, dirY = -1, -1
	}

	for rowIdx, ring := range cfg.KeyRings {
		keysInRow := len(ring)
		rowWidth := float32(keysInRow) * cellSize
		startX := (availW - rowWidth) / 2.0

		for colIdx, char := range ring {
			x := (startX + float32(colIdx)*cellSize + cellSize/2.0) * dirX
			y := (float32(rowIdx)*cellSize + cellSize/2.0) * dirY

			keys = append(keys, KeyDefinition{
				Char:    char,
				X:       x,
				Y:       y,
				KeySize: keySize,
			})
		}
	}
	return keys
}

// GetLayout returns the layout implementation based on style string
func GetLayout(style string) KeyboardLayout {
	switch style {
	case "grid":
		return &GridLayout{}
	default:
		return &FanLayout{}
	}
}

// GetExpandedSize calculates required window dimensions to accommodate scaled layout bounds
func (cfg *Config) GetExpandedSize() int {
	var maxRadius float32
	if cfg.LayoutStyle == "grid" {
		cols := float32(6)
		rows := float32(3)
		cellSize := cfg.KeySize + cfg.KeyGap
		maxRadius = float32(math.Sqrt(float64(cols*cols+rows*rows)))*cellSize + 30
	} else {
		maxRadius = cfg.FanConfig.OuterRadius + cfg.KeySize
	}

	pad := cfg.Padding.Right
	if cfg.Padding.Left > pad {
		pad = cfg.Padding.Left
	}
	if cfg.Padding.Top > pad {
		pad = cfg.Padding.Top
	}
	if cfg.Padding.Bottom > pad {
		pad = cfg.Padding.Bottom
	}

	sizeDp := int((maxRadius+pad)*cfg.KeyboardScale)
	if sizeDp < 300 {
		return 300
	}
	return sizeDp
}

func (cfg *Config) GetExpandedWidth() int {
	if cfg.ExpandedWidth > 0 {
		return cfg.ExpandedWidth
	}
	return cfg.GetExpandedSize()
}

func (cfg *Config) GetExpandedHeight() int {
	if cfg.ExpandedHeight > 0 {
		return cfg.ExpandedHeight
	}
	return cfg.GetExpandedSize()
}

// WindowPositionForCorner returns the top-left screen coordinates for the window
func (cfg *Config) WindowPositionForCorner(screenSize image.Point, w, h int) (int, int) {
	switch cfg.ScreenPosition {
	case "top-left":
		return 0, 0
	case "top-right":
		return screenSize.X - w, 0
	case "bottom-left":
		return 0, screenSize.Y - h
	default: // bottom-right
		return screenSize.X - w, screenSize.Y - h
	}
}

