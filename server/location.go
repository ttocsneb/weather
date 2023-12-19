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
	"github.com/ttocsneb/weather/stations"
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
			map_ids := make([]string, len(stations))
			for i, station := range stations {
				mapid := station.MapId()
				mapping[mapid] = i
				map_ids[i] = mapid
			}

			entries, err := fetchConditions(db, stations)
			if err != nil {
				ErrorMessage(w, 500, "Internal Server Error")
				fmt.Printf("Unable to fetch weather entries: %v\n", err)
				return
			}

			if len(stations) == 1 {
				// There is only one station, there is no need to do a weighted average
				values := make(map[string]types.SensorValue)

				for name, sensors := range entries[0].Sensors {
					val, unit := util.SensorToMetric(sensors[0].Value, sensors[0].Unit, name)
					values[name] = types.SensorValue{
						Unit:  unit,
						Value: val,
					}
				}

				for name, sensor := range values {
					value, unit := util.SensorToImperial(sensor.Value, sensor.Unit, name)
					values[name] = types.SensorValue{
						Unit:  unit,
						Value: value,
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
			weight_mappings := make(map[string]float64)
			for i, weight := range weights {
				weight_mappings[map_ids[i]] = weight
			}
			average_values := averageConditions(entries, weight_mappings)

			for name, sensor := range average_values {
				value, unit := util.SensorToImperial(sensor.Value, sensor.Unit, name)
				average_values[name] = types.SensorValue{
					Unit:  unit,
					Value: value,
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

func fetchConditions(db *sql.DB, stations []types.StationEntry) ([]types.WeatherEntry, error) {
	conditions := make([]string, len(stations))
	args := make([]interface{}, len(stations)*4)

	for i, station := range stations {
		conditions[i] = fmt.Sprintf(
			`(server.value = ? AND station.value = ? AND time = (
			SELECT MAX(time) FROM weather_entry %v
			WHERE server.value = ? AND station.value = ?)
		)`, database.GenStringJoins("weather_entry", "station", "server"))
		args[i*4] = station.Server
		args[i*4+1] = station.Station
		args[i*4+2] = station.Server
		args[i*4+3] = station.Station
	}

	entries, err := database.FetchEntries(db,
		fmt.Sprintf("WHERE %v", strings.Join(conditions, " OR ")), args...)
	return entries, err
}

func averageConditions(conditions []types.WeatherEntry, weights map[string]float64) map[string]types.SensorValue {
	type foo struct {
		unit    string
		values  []float64
		weights []float64
	}
	value_list := make(map[string]foo)
	average_values := make(map[string]types.SensorValue)

	for _, entry := range conditions {
		weight := weights[entry.MapId()]
		for name, sensors := range entry.Sensors {
			val, unit := util.SensorToMetric(sensors[0].Value, sensors[0].Unit, name)
			v, exists := value_list[name]
			if !exists {
				v = foo{
					unit:    unit,
					values:  []float64{},
					weights: []float64{},
				}
			}
			v.values = append(v.values, val)
			v.weights = append(v.weights, weight)
			value_list[name] = v
		}
	}

	for key, values := range value_list {
		average_values[key] = types.SensorValue{
			Value: util.AverageSensor(
				values.values, values.weights, values.unit, key),
			Unit: values.unit,
		}
	}

	return average_values
}

func LocationConditionsUpdateRoute(db *sql.DB, brokers map[string]stations.Broker, r *mux.Router) {
	r.HandleFunc("/location/conditions/updates/",
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

			// Subscribe to each station's raw_updates in this location
			raw_updates := make(map[string]chan types.WeatherMessage)
			updates := make(chan types.WeatherEntry)

			on_update := func(server string, ch chan types.WeatherMessage) {
				for {
					update, ok := <-ch
					if !ok {
						break
					}
					updates <- update.ToEntry(server)
				}
			}

			for _, station := range stations {
				broker, exists := brokers[station.Server]
				if !exists {
					ErrorMessage(w, 500, "Internal Server Error")
					fmt.Printf("Couldn't find broker %s\n", station.Server)
					return
				}

				ch, exists := raw_updates[station.Server]
				if !exists {
					ch = make(chan types.WeatherMessage)
					raw_updates[station.Server] = ch
					go on_update(station.Server, ch)
				}

				broker.SubscribeWeatherUpdates(station.Station, ch)
			}
			defer func() {
				for _, station := range stations {
					broker := brokers[station.Server]
					ch := raw_updates[station.Server]
					broker.UnsubscribeWeatherUpdates(station.Station, ch)
				}
				for _, ch := range raw_updates {
					close(ch)
				}
				close(updates)
			}()

			conditions := make(map[string]types.WeatherEntry)

			// Fetch the current weather conditions for the location
			mapping := make(map[string]int)
			map_ids := make([]string, len(stations))
			for i, station := range stations {
				mapid := station.MapId()
				mapping[mapid] = i
				map_ids[i] = mapid
			}
			entries, err := fetchConditions(db, stations)
			if err != nil {
				ErrorMessage(w, 500, "Internal Server Error")
				fmt.Printf("Unable to fetch weather entries: %v\n", err)
				return
			}
			for _, entry := range entries {
				conditions[entry.MapId()] = entry
			}
			weights := findWeights(&dists)
			weight_mapping := make(map[string]float64)
			for i, weight := range weights {
				weight_mapping[map_ids[i]] = weight
			}

			updateConditions := func() {
				entries := make([]types.WeatherEntry, len(conditions))
				i := 0
				for _, entry := range conditions {
					entries[i] = entry
				}

				vals := averageConditions(entries, weight_mapping)
				for name, sensor := range vals {
					value, unit := util.SensorToImperial(sensor.Value, sensor.Unit, name)
					vals[name] = types.SensorValue{
						Unit:  unit,
						Value: value,
					}
				}

				data, err := json.Marshal(vals)
				if err != nil {
					ErrorMessage(w, 500, "Internal Server Error")
					fmt.Printf("Unable to marshal weather conditions")
					return
				}

				w.Write([]byte(fmt.Sprintf("data: %v\n\n", string(data))))
				w.(http.Flusher).Flush()
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(200)
			w.(http.Flusher).Flush()

			updateConditions()

			// Whenever an update comes, recalculate the weather conditions for this location
			for {
				select {
				case update := <-updates:
					conditions[update.MapId()] = update
					updateConditions()
				case <-r.Context().Done():
					return
				}
			}
		})
}
