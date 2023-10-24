package util

import (
	"math"
	"strings"
)

func HarvesineDistance(lata float64, lona float64, latb float64, lonb float64) float64 {
	lata *= math.Pi / 180
	latb *= math.Pi / 180
	lona *= math.Pi / 180
	lonb *= math.Pi / 180

	Δlat := latb - lata
	Δlon := lonb - lona

	sin_lat := math.Sin(Δlat / 2)
	sin_lon := math.Sin(Δlon / 2)
	a := sin_lat*sin_lat + math.Cos(lata)*math.Cos(latb)*sin_lon*sin_lon
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return c * 6371
}

func DistToLatLon(lon float64, dist float64) (float64, float64) {
	const r float64 = 6371

	ratio := dist / r

	return ratio * (180 / math.Pi),
		math.Abs(ratio * (180 / math.Pi) / math.Cos(lon*(math.Pi/180)))
}

func NormalizeSensor(value float64, unit string, sensor_hint string) (float64, string) {
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

func AverageWeights(values []float64, weights []float64) float64 {
	total := 0.
	weight_sum := 0.
	for i, value := range values {
		weight := weights[i]
		total += value * weight
		weight_sum += weight
	}

	return total / weight_sum
}

func RadsToUnit(angles []float64) ([]float64, []float64) {
	sins := make([]float64, len(angles))
	coss := make([]float64, len(angles))

	for i, angle := range angles {
		sins[i] = math.Sin(angle)
		coss[i] = math.Cos(angle)
	}

	return sins, coss
}

func UnitToRad(sin float64, cos float64) float64 {
	return math.Atan2(sin, cos)
}

func ToRads(degs []float64) []float64 {
	rads := make([]float64, len(degs))
	for i, deg := range degs {
		rads[i] = deg * math.Pi / 180.0
	}
	return rads
}

func ToDegs(rads []float64) []float64 {
	degs := make([]float64, len(rads))
	for i, rad := range rads {
		degs[i] = rad * 180.0 / math.Pi
	}
	return degs
}

func ModBounds(value float64, max float64) float64 {
	for value > max {
		value -= max
	}
	for value < 0 {
		value += max
	}
	return value
}

func AverageSensor(values []float64, weights []float64, unit string, sensor_hint string) float64 {
	switch unit {
	case "deg":
		rads := ToRads(values)
		sins, coss := RadsToUnit(rads)
		sin := AverageWeights(sins, weights)
		cos := AverageWeights(coss, weights)
		rad := UnitToRad(sin, cos)
		deg := rad * 180.0 / math.Pi
		return ModBounds(deg, 360)
	case "rad":
		sins, coss := RadsToUnit(values)
		sin := AverageWeights(sins, weights)
		cos := AverageWeights(coss, weights)
		rad := UnitToRad(sin, cos)
		return ModBounds(rad, 2*math.Pi)
	}
	return AverageWeights(values, weights)
}

func IndexOf[T comparable](list []T, value *T) (int, bool) {
	for i, val := range list {
		if val == *value {
			return i, true
		}
	}
	return 0, false
}
