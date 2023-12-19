package util

import (
	"math"
	"strings"
)

func SensorToMetric(value float64, unit string, sensor_hint string) (float64, string) {
	unit = strings.ToLower(unit)
	switch sensor_hint {
	case "uv":
		unit = "uv"
	}
	switch unit {
	case "m/h", "mph":
		unit = "mps"
		value = value / 2.236936
	case "mps", "m/s":
		unit = "mps"
	case "c":
		unit = "c"
	case "f":
		unit = "c"
		value = (value - 32) * 5.0 / 9.0
	case "in":
		unit = "mm"
		value = value / 25.4
	case "mm":
		unit = "mm"
	case "nm":
		unit = "km"
		value = value * 1.852
	case "mi":
		unit = "km"
		value = value * 1.609344
	case "km":
		unit = "km"
	case "inhg":
		unit = "hpa"
		value = value * 33.86389
	case "torr":
		unit = "hpa"
		value = value * 1.3332236842105
	case "hpa":
		unit = "hpa"
	case "%", "pct":
		unit = "%"
	case "deg":
		unit = "deg"
	case "rad":
		unit = "deg"
		value = value * 180.0 / math.Pi
	}
	return value, unit
}

func SensorToImperial(value float64, unit string, sensor_hint string) (float64, string) {
	unit = strings.ToLower(unit)
	switch unit {
	case "m/s", "mps":
		unit = "mph"
		value = value * 2.236936
	case "c":
		unit = "f"
		value = value*9.0/5.0 + 32
	case "f":
		unit = "f"
	case "in":
		unit = "in"
	case "mm":
		unit = "in"
		value = value * 25.4
	case "nm":
		unit = "nm"
	case "mi":
		unit = "mi"
	case "km":
		unit = "mi"
		value = value / 1.609344
	case "inhg":
		unit = "inhg"
	case "torr":
		unit = "inhg"
		value = value / 25.4
	case "hpa":
		unit = "inhg"
		value = value / 33.86389
	case "%", "pct":
		unit = "%"
	case "deg":
		unit = "deg"
	case "rad":
		unit = "deg"
		value = value * 180.0 / math.Pi
	}
	return value, unit
}
