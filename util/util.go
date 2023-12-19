package util

import (
	"math"
	"strconv"
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

func DecodeURIString(data string) (string, error) {
	data = strings.ReplaceAll(data, "+", " ")

	var builder strings.Builder

	i := 0
	for i < len(data) {
		if data[i] == '%' {
			var code string
			if data[i+1] == 'u' || data[i+1] == 'U' {
				code = data[i+2 : i+2+4]
				i += 6
			} else {
				code = data[i+1 : i+1+2]
				i += 3
			}
			val, err := strconv.ParseInt(code, 32, 0)
			if err != nil {
				return "", err
			}
			_, err = builder.WriteRune(rune(val))
			if err != nil {
				return "", err
			}
		} else {
			err := builder.WriteByte(data[i])
			if err != nil {
				return "", err
			}
			i += 1
		}
	}

	return builder.String(), nil
}

func EncodeURIString(data string) string {
	var builder strings.Builder

	for i := 0; i < len(data); i++ {
		c := data[i]
		switch {
		case c == ' ':
			builder.WriteRune('+')
		case (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') &&
			c != '-' && c != '_' && c != '.' && c != '~':
			builder.WriteRune('%')
			builder.WriteString(strconv.FormatInt(int64(c), 16))
		default:
			builder.WriteByte(c)
		}
	}
	return builder.String()
}
