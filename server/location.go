package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/ttocsneb/weather/database"
	"github.com/ttocsneb/weather/types"
	"github.com/ttocsneb/weather/util"
)

func NearestStationRoute(db *sql.DB, r *mux.Router) {
	r.HandleFunc("/location/nearest/",
		func(w http.ResponseWriter, r *http.Request) {
			var err error
			w.Header().Set("Cache-Control", "no-cache")

			q := r.URL.Query()

			if !q.Has("lat") || !q.Has("lon") {
				ErrorMessage(w, 400, "lat and lon are required parameters")
				return
			}

			lat_s := q.Get("lat")
			lon_s := q.Get("lon")
			dist := 15.0

			if q.Has("range") {
				dist_s := q.Get("range")
				dist, err = strconv.ParseFloat(dist_s, 64)
				if err != nil {
					ErrorMessage(w, 400, "range must be a number")
					return
				}
			}

			lat, err := strconv.ParseFloat(lat_s, 64)
			if err != nil {
				ErrorMessage(w, 400, "lat must be a number")
				return
			}
			lon, err := strconv.ParseFloat(lon_s, 64)
			if err != nil {
				ErrorMessage(w, 400, "lon must be a number")
				return
			}

			delta_lat, delta_lon := util.DistToLatLon(lon, dist)

			// TODO There is a bug where longitude will not wrap around in this
			// initial check
			entries, err := database.QueryStationInfos(db, `WHERE
				latitude BETWEEN ? AND ?
				AND longitude BETWEEN ? AND ?`,
				lat-delta_lat, lat+delta_lat,
				lon-delta_lon, lon+delta_lon,
			)
			if err != nil {
				ErrorMessage(w, 404, "Internal Server Error")
				fmt.Printf("Error while fetching nearest station: %v\n", err)
				return
			}

			if len(entries) == 0 {
				ErrorMessage(w, 404, "Entry not found")
				return
			}

			closest := entries[0]
			closest_dist := util.HarvesineDistance(lat, lon,
				entries[0].Latitude, entries[0].Longitude)

			for _, entry := range entries[1:] {
				d := util.HarvesineDistance(lat, lon,
					entry.Latitude, entry.Longitude)
				if d < closest_dist {
					closest_dist = d
					closest = entry
				}
			}

			fmt.Printf("dist: %v\n", closest_dist)

			data, err := json.Marshal(closest)
			if err != nil {
				ErrorMessage(w, 500, "Internal Server Error")
				fmt.Printf("Could not marshal closest entry: %v\n", err)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		})
}

func findNearestStations(db *sql.DB, lat float64, lon float64, dist float64) ([]types.StationEntry, []float64, error) {
	stations := []types.StationEntry{}
	distances := []float64{}

	delta_lat, delta_lon := util.DistToLatLon(lon, dist)

	entries, err := database.QueryStationInfos(db, `WHERE
				latitude BETWEEN ? AND ?
				AND longitude BETWEEN ? AND ?`,
		lat-delta_lat, lat+delta_lat,
		lon-delta_lon, lon+delta_lon,
	)
	if err != nil {
		return stations, distances, err
	}

	for _, entry := range entries {
		d := util.HarvesineDistance(lat, lon, entry.Latitude, entry.Longitude)
		if d <= dist {
			stations = append(stations, entry)
			distances = append(distances, d)
		}
	}

	return stations, distances, nil
}

func findWeights(stations *[]float64) []float64 {
	total_distances := 0.0

	for _, dist := range *stations {
		total_distances += dist
	}

	normalized := make([]float64, len(*stations))
	for i, dist := range *stations {
		normalized[i] = dist / total_distances
	}

	weights := make([]float64, len(*stations))
	total_weighted_sum := 0.0
	for i, n := range normalized {
		weights[i] = 1.0 / (n + 1)
		total_weighted_sum += weights[i]
	}

	normalized_weights := normalized
	for i, w := range weights {
		normalized_weights[i] = w / total_weighted_sum
	}

	return normalized_weights
}

func LocationConditionsRoute(db *sql.DB, r *mux.Router) {
	r.HandleFunc("/location/conditions/",
		func(w http.ResponseWriter, r *http.Request) {
			var err error
			w.Header().Set("Cache-Control", "no-cache")

			q := r.URL.Query()

			if !q.Has("lat") || !q.Has("lon") {
				ErrorMessage(w, 400, "lat and lon are required parameters")
				return
			}

			lat_s := q.Get("lat")
			lon_s := q.Get("lon")
			dist := 15.0

			if q.Has("range") {
				dist_s := q.Get("range")
				dist, err = strconv.ParseFloat(dist_s, 64)
				if err != nil {
					ErrorMessage(w, 400, "range must be a number")
					return
				}
			}

			lat, err := strconv.ParseFloat(lat_s, 64)
			if err != nil {
				ErrorMessage(w, 400, "lat must be a number")
				return
			}
			lon, err := strconv.ParseFloat(lon_s, 64)
			if err != nil {
				ErrorMessage(w, 400, "lon must be a number")
				return
			}

			stations, dists, err := findNearestStations(db, lat, lon, dist)
			if err != nil {
				ErrorMessage(w, 500, "Internal Server Error")
				fmt.Printf("Unable to find nearest stations: %v\n", err)
				return
			}
			if len(stations) == 0 {
				ErrorMessage(w, 404, "No stations found")
				return
			}

			mapping := make(map[string]int)

			conditions := make([]string, len(stations))
			args := make([]interface{}, len(stations)*4)
			for i, station := range stations {
				// TODO make it so that time can't be too old
				conditions[i] = fmt.Sprintf(
					`(server.value = ? AND station.value = ? AND time = (
					SELECT MAX(time) FROM weather_entry %v
					WHERE server.value = ? AND station.value = ?)
					)`, database.GenQueryLookup("weather_entry", "station", "server"))
				args[i*4] = station.Server
				args[i*4+1] = station.Station
				args[i*4+2] = station.Server
				args[i*4+3] = station.Station
				mapping[fmt.Sprintf("%v-%v", station.Server, station.Station)] = i
			}

			entries, err := database.FetchEntries(db,
				fmt.Sprintf("WHERE %v", strings.Join(conditions, " OR ")), args...)
			if err != nil {
				ErrorMessage(w, 500, "Internal Server Error")
				fmt.Printf("Unable to fetch weather entries: %v\n", err)
				return
			}

			if len(stations) == 1 {
				// There is only one station, there is no need to do a weighted average
				values := make(map[string]types.SensorValue)

				for name, sensors := range entries[0].Sensors {
					val, unit := util.NormalizeSensor(sensors[0].Value, sensors[0].Unit, name)
					values[name] = types.SensorValue{
						Unit:  unit,
						Value: val,
					}
				}

				data, err := json.Marshal(values)
				if err != nil {
					ErrorMessage(w, 500, "Internal Server Error")
					fmt.Printf("Could not marshal conditions: %v\n", err)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.Write(data)
				return
			}

			weights := findWeights(&dists)

			average_values := make(map[string]types.SensorValue)

			i := mapping[fmt.Sprintf("%v-%v", entries[0].Server, entries[0].Station)]
			weight := weights[i]
			for name, sensors := range entries[0].Sensors {
				val, unit := util.NormalizeSensor(sensors[0].Value, sensors[0].Unit, name)
				average_values[name] = types.SensorValue{
					Unit:  unit,
					Value: util.AverageSensor(val, weight, unit, name),
				}
			}

			for _, entry := range entries[1:] {
				i := mapping[fmt.Sprintf("%v-%v", entry.Server, entry.Station)]
				weight := weights[i]
				to_remove := []string{}
				for name, sensor := range average_values {
					sensors, exists := entry.Sensors[name]
					if !exists {
						to_remove = append(to_remove, name)
						continue
					}
					val, unit := util.NormalizeSensor(sensors[0].Value, sensors[0].Unit, name)
					average_values[name] = types.SensorValue{
						Unit:  sensor.Unit,
						Value: sensor.Value + util.AverageSensor(val, weight, unit, name),
					}
				}
				for _, name := range to_remove {
					delete(average_values, name)
				}
			}

			data, err := json.Marshal(average_values)
			if err != nil {
				ErrorMessage(w, 500, "Internal Server Error")
				fmt.Printf("Could not marshal conditions: %v\n", err)
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		})
}
