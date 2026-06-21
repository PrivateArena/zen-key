package main

import "math"

type KeyDefinition struct {
	Char     string
	X, Y     float32 // Render offsets calculated relative to pivot
	Radius   float32
	AngleDeg float32
}

type Config struct {
	InnerRadius float32
	OuterRadius float32
	StartAngle  float32 // 180 degrees (Left-facing horizon)
	EndAngle    float32 // 270 degrees (Upward vertical)
	TriggerSize float32 // Bottom-right corner trigger box size in pixels
}

var DefaultConfig = Config{
	InnerRadius: 70.0,
	OuterRadius: 230.0,
	StartAngle:  180.0,
	EndAngle:    270.0,
	TriggerSize: 15.0,
}

// Optimized 3-tier Zen cluster mapping by linguistic usage frequency
var KeyRings = [][]string{
	{"E", "T", "A", "O", "I", "N"}, // Inner Ring (High frequency)
	{"S", "H", "R", "D", "L", "C"}, // Middle Ring
	{"U", "M", "W", "F", "G", "Y"}, // Outer Ring
}

// CalculateFanKeys computes polar coordinates mapping down to the bottom-right pivot
func CalculateFanKeys(cfg Config) []KeyDefinition {
	var keys []KeyDefinition
	totalRings := len(KeyRings)

	for ringIdx, ring := range KeyRings {
		var radius float32
		if totalRings > 1 {
			radius = cfg.InnerRadius + (float32(ringIdx)/float32(totalRings-1))*(cfg.OuterRadius-cfg.InnerRadius)
		} else {
			radius = cfg.InnerRadius
		}

		keysInRing := len(ring)
		for charIdx, char := range ring {
			var angleDeg float32
			if keysInRing > 1 {
				angleDeg = cfg.StartAngle + (float32(charIdx)/float32(keysInRing-1))*(cfg.EndAngle-cfg.StartAngle)
			} else {
				angleDeg = cfg.StartAngle
			}

			angleRad := float64(angleDeg * math.Pi / 180.0)
			
			// Polar to Cartesian coordinate mapping relative to bottom-right corner
			x := radius * float32(math.Cos(angleRad))
			y := radius * float32(math.Sin(angleRad))

			keys = append(keys, KeyDefinition{
				Char:     char,
				X:        x,
				Y:        y,
				Radius:   radius,
				AngleDeg: angleDeg,
			})
		}
	}
	return keys
}
