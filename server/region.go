package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/ttocsneb/weather/database"
	"github.com/ttocsneb/weather/stations"
	"github.com/ttocsneb/weather/types"
)

func findRegionStations(db *sql.DB, district *string, city *string, region *string, country *string) ([]types.StationEntry, error) {
	conditions := []string{}
	args := []interface{}{}
	if district != nil {
		conditions = append(conditions, fmt.Sprintf("district.value = ?"))
		args = append(args, *district)
	}
	if city != nil {
		conditions = append(conditions, fmt.Sprintf("city.value = ?"))
		args = append(args, *city)
	}
	if region != nil {
		conditions = append(conditions, fmt.Sprintf("region.value = ?"))
		args = append(args, *region)
	}
	if country != nil {
		conditions = append(conditions, fmt.Sprintf("country.value = ?"))
		args = append(args, *country)
	}
	query := fmt.Sprintf(`WHERE %v`, strings.Join(conditions, " AND "))

	return database.QueryStationInfos(db, query, args...)
}

func searchRegion(db *sql.DB, district *string, city *string, region *string, country *string) (int, error) {
	conditions := []string{}
	args := []interface{}{}
	if district != nil {
		conditions = append(conditions, fmt.Sprintf("district.value = ?"))
		args = append(args, *district)
	}
	if city != nil {
		conditions = append(conditions, fmt.Sprintf("city.value = ?"))
		args = append(args, *city)
	}
	if region != nil {
		conditions = append(conditions, fmt.Sprintf("region.value = ?"))
		args = append(args, *region)
	}
	if country != nil {
		conditions = append(conditions, fmt.Sprintf("country.value = ?"))
		args = append(args, *country)
	}
	query := fmt.Sprintf(
		`SELECT DISTINCT district.value, city.value, region.value, country.value 
		FROM station 
		%v 
		WHERE %v;`,
		database.GenStringJoins(
			"station", "district", "city", "region", "country"),
		strings.Join(conditions, " AND "))

	rows, err := db.Query(query, args...)
	if err != nil {
		return 0, err
	}

	count := 0
	for rows.Next() {
		count += 1
	}

	return count, nil
}

func RegionConditionsRoute(db *sql.DB, r *mux.Router) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")

		query := r.URL.Query()
		var country *string = nil
		if query.Has("country") {
			country_var := query.Get("country")
			country = &country_var
		}
		var region *string = nil
		if query.Has("region") {
			region_var := query.Get("region")
			region = &region_var
		}
		var city *string = nil
		if query.Has("city") {
			city_var := query.Get("city")
			city = &city_var
		}
		var district *string = nil
		if query.Has("district") {
			district_var := query.Get("district")
			district = &district_var
		}

		locations, err := searchRegion(db, district, city, region, country)
		if err != nil {
			ErrorMessage(w, 500, "Internal Server Error")
			fmt.Printf("Could not search region: %v\n", err)
			return
		}
		if locations > 1 {
			ErrorMessage(w, 400, "Region is ambiguous, be more specific")
			return
		}
		if locations == 0 {
			ErrorMessage(w, 404, "Region not found")
			return
		}

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
	r.HandleFunc("/region/conditions/",
		handler)
}

func RegionConditionsUpdateRoute(db *sql.DB, brokers map[string]stations.Broker, r *mux.Router) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")

		query := r.URL.Query()
		var country *string = nil
		if query.Has("country") {
			country_var := query.Get("country")
			country = &country_var
		}
		var region *string = nil
		if query.Has("region") {
			region_var := query.Get("region")
			region = &region_var
		}
		var city *string = nil
		if query.Has("city") {
			city_var := query.Get("city")
			city = &city_var
		}
		var district *string = nil
		if query.Has("district") {
			district_var := query.Get("district")
			district = &district_var
		}

		locations, err := searchRegion(db, district, city, region, country)
		if err != nil {
			ErrorMessage(w, 500, "Internal Server Error")
			fmt.Printf("Could not search region: %v\n", err)
			return
		}
		if locations > 1 {
			ErrorMessage(w, 400, "Region is ambiguous, be more specific")
			return
		}
		if locations == 0 {
			ErrorMessage(w, 404, "Region not found")
			return
		}

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
	r.HandleFunc("/region/conditions/updates/",
		handler)
}
