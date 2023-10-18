package util

import "math"

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

func NormalizeSensor(value float32, unit string, sensor_hint string) (float32, string) {
	return value, unit
	// TODO, separate by unit type, then convert to a standard unit
}

func AverageSensor(value float32, weight float64, unit string, sensor_hint string) float32 {
	return value * float32(weight)
}

func FinalizeAverageSensor(value float32, unit string, sensor_hint string) float32 {
	return value
}
