package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/ttocsneb/weather/database"
	"github.com/ttocsneb/weather/stations"
	"github.com/ttocsneb/weather/types"
	"github.com/ttocsneb/weather/util"
)

func findRegionStations(db *sql.DB, district string, city string, region string, country string) ([]types.StationEntry, error) {
	conditions := []string{}
	args := []interface{}{}
	if district != "" {
		conditions = append(conditions, fmt.Sprintf("district.value LIKE ?"))
		args = append(args, district)
	}
	if city != "" {
		conditions = append(conditions, fmt.Sprintf("city.value LIKE ?"))
		args = append(args, city)
	}
	if region != "" {
		conditions = append(conditions, fmt.Sprintf("region.value LIKE ?"))
		args = append(args, region)
	}
	if country != "" {
		conditions = append(conditions, fmt.Sprintf("country.value LIKE ?"))
		args = append(args, country)
	}
	query := fmt.Sprintf(`WHERE %v`, strings.Join(conditions, " AND "))

	return database.QueryStationInfos(db, query, args...)
}

type searchResult struct {
	Country  string `json:"country"`
	Region   string `json:"region"`
	City     string `json:"city"`
	District string `json:"district"`
}

func searchForRegion(db *sql.DB, vals ...string) ([]searchResult, error) {
	args := []any{}
	conditions := []string{}

	option := func(country string, region string, city string, district string) {
		region_conds := []string{}

		if country != "" {
			region_conds = append(region_conds, fmt.Sprintf("country.value LIKE ?"))
			args = append(args, country)
		}
		if region != "" {
			region_conds = append(region_conds, fmt.Sprintf("region.value LIKE ?"))
			args = append(args, region)
		}
		if city != "" {
			region_conds = append(region_conds, fmt.Sprintf("city.value LIKE ?"))
			args = append(args, city)
		}
		if district != "" {
			region_conds = append(region_conds, fmt.Sprintf("district.value LIKE ?"))
			args = append(args, district)
		}

		conditions = append(conditions, fmt.Sprintf("( %v )", strings.Join(region_conds, " AND ")))
	}

	if len(vals) == 1 {
		option("", "", vals[0], "")
		option("", "", "", vals[0])
	} else if len(vals) == 2 {
		option(vals[0], "", vals[1], "")
		option(vals[0], "", "", vals[1])
		option("", vals[0], vals[1], "")
		option("", vals[0], "", vals[1])
		option("", "", vals[0], vals[1])

		option(vals[1], "", vals[0], "")
		option(vals[1], "", "", vals[0])
		option("", vals[1], vals[0], "")
		option("", vals[1], "", vals[0])
		option("", "", vals[1], vals[0])
	} else if len(vals) == 3 {
		option(vals[0], vals[1], vals[2], "")
		option(vals[0], vals[1], "", vals[2])
		option(vals[0], "", vals[1], vals[2])
		option("", vals[0], vals[1], vals[2])

		option(vals[2], vals[1], vals[0], "")
		option(vals[2], vals[1], "", vals[0])
		option(vals[2], "", vals[1], vals[0])
		option("", vals[2], vals[1], vals[0])
	} else if len(vals) == 4 {
		option(vals[0], vals[1], vals[2], vals[4])
		option(vals[3], vals[2], vals[1], vals[0])
	} else {
		return nil, errors.New("Must have between 1 and 4 value parameters")
	}

	query := fmt.Sprintf(
		`SELECT DISTINCT country.value, region.value, city.value, district.value 
		FROM station 
		%v 
		WHERE %v;`,
		database.GenStringJoins(
			"station", "district", "city", "region", "country"),
		strings.Join(conditions, " OR "))

	fmt.Println(query)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}

	output := []searchResult{}
	for rows.Next() {
		var country string
		var region string
		var city string
		var district string
		err = rows.Scan(&country, &region, &city, &district)
		if err != nil {
			return nil, err
		}

		output = append(output, searchResult{country, region, city, district})
	}

	return output, nil
}

func RegionSearchRoute(db *sql.DB, r *mux.Router) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		items := []string{}

		a, _ := util.DecodeURIString(query.Get("a"))
		if a != "" {
			items = append(items, a)
		}
		b, _ := util.DecodeURIString(query.Get("b"))
		if b != "" {
			items = append(items, b)
		}
		c, _ := util.DecodeURIString(query.Get("c"))
		if c != "" {
			items = append(items, c)
		}
		d, _ := util.DecodeURIString(query.Get("d"))
		if d != "" {
			items = append(items, d)
		}

		regions, err := searchForRegion(db, items...)
		if err != nil {
			ErrorMessage(w, 500, "Internal Server Error")
			fmt.Printf("Could not search region: %v\n", err)
			return
		}

		data, err := json.Marshal(regions)
		if err != nil {
			ErrorMessage(w, 500, "Internal Server Error")
			fmt.Printf("Could not marshal search region: %v\n", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(data)
	}
	r.HandleFunc("/region/search/",
		handler)
}

func RegionConditionsRoute(db *sql.DB, r *mux.Router) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")

		vars := mux.Vars(r)

		country, _ := util.DecodeURIString(vars["country"])
		region, _ := util.DecodeURIString(vars["region"])
		city, _ := util.DecodeURIString(vars["city"])
		district, _ := util.DecodeURIString(vars["district"])

		stations, err := findRegionStations(db, district, city, region, country)
		if len(stations) == 0 {
			ErrorMessage(w, 404, "Region not found")
			return
		}
		if err != nil {
			ErrorMessage(w, 500, "Internal Server Error")
			fmt.Printf("Could not fetch region stations: %v\n", err)
			return
		}

		entries, err := fetchConditions(db, stations)
		if err != nil {
			ErrorMessage(w, 500, "Internal Server Error")
			fmt.Printf("Could not fetch region entries: %v\n", err)
			return
		}

		weight := 1.0 / float64(len(entries))
		weight_map := make(map[string]float64)
		for _, station := range stations {
			weight_map[station.MapId()] = weight
		}

		results := averageConditions(entries, weight_map)
		for name, sensor := range results {
			value, unit := util.SensorToImperial(sensor.Value, sensor.Unit, name)
			results[name] = types.SensorValue{
				Unit:  unit,
				Value: value,
			}
		}

		data, err := json.Marshal(results)
		if err != nil {
			ErrorMessage(w, 500, "Internal Server Error")
			fmt.Printf("Could not marshal region entries: %v\n", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(data)
	}

	r.HandleFunc("/region/conditions/{country}/{region}/{city}/", handler)
	r.HandleFunc("/region/conditions/{country}/{region}/{city}/{district}/", handler)
}

func RegionConditionsUpdateRoute(db *sql.DB, brokers map[string]stations.Broker, r *mux.Router) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")

		vars := mux.Vars(r)

		country, _ := util.DecodeURIString(vars["country"])
		region, _ := util.DecodeURIString(vars["region"])
		city, _ := util.DecodeURIString(vars["city"])
		district, _ := util.DecodeURIString(vars["district"])

		stations, err := findRegionStations(db, district, city, region, country)
		if len(stations) == 0 {
			ErrorMessage(w, 404, "Region not found")
			return
		}
		if err != nil {
			ErrorMessage(w, 500, "Internal Server Error")
			fmt.Printf("Could not fetch region stations: %v\n", err)
			return
		}

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
		weight := 1.0 / float64(len(entries))
		weight_map := make(map[string]float64)
		for _, station := range stations {
			weight_map[station.MapId()] = weight
		}

		updateConditions := func() {
			entries := make([]types.WeatherEntry, len(conditions))
			i := 0
			for _, entry := range conditions {
				entries[i] = entry
			}

			vals := averageConditions(entries, weight_map)
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

		for {
			select {
			case update := <-updates:
				conditions[update.MapId()] = update
				updateConditions()
			case <-r.Context().Done():
				return
			}
		}
	}
	r.HandleFunc("/region/conditions/updates/{country}/{region}/{city}/", handler)
	r.HandleFunc("/region/conditions/updates/{country}/{region}/{city}/{district}/", handler)
}
