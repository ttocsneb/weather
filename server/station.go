package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/ttocsneb/weather/database"
	"github.com/ttocsneb/weather/stations"
	"github.com/ttocsneb/weather/types"
)

func StationConditionsRoute(db *sql.DB, r *mux.Router) {
	r.HandleFunc("/station/{server}/{station}/conditions/",
		func(w http.ResponseWriter, r *http.Request) {
			vars := mux.Vars(r)
			server := vars["server"]
			station := vars["station"]

			w.Header().Set("Cache-Control", "no-cache")

			entry, err := database.FetchEntry(db, `WHERE server.value = ? 
					AND station.value = ?
				ORDER BY time DESC`,
				server, station)
			if err != nil {
				ErrorMessage(w, 404, "Station not found")
				fmt.Printf("Could not fetch entry: %v\n", err)
				return
			}

			data, err := json.Marshal(entry)
			if err != nil {
				ErrorMessage(w, 500, "Could not encode entry")
				fmt.Printf("Could not encode entry: %v\n", err)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		})
}

func StationHistoryRoute(db *sql.DB, r *mux.Router) {
	r.HandleFunc("/station/{server}/{station}/conditions/history/",
		func(w http.ResponseWriter, r *http.Request) {
			vars := mux.Vars(r)
			server := vars["server"]
			station := vars["station"]

			w.Header().Set("Cache-Control", "no-cache")

			query := r.URL.Query()
			before := query.Get("before")
			after := query.Get("after")
			count := query.Get("count")
			order := query.Get("order")

			fmt.Println(before)
			fmt.Println(after)
			fmt.Println(count)
			fmt.Println(order)

			args := []interface{}{
				server, station,
			}

			if order == "asc" {
				order = "ASC"
			} else {
				order = "DESC"
			}

			before_t, err := time.Parse("2006-01-02T15:04:05Z07:00", before)
			before_query := ""
			if err == nil {
				before_query = `AND time <= ?`
				args = append(args, before_t)
			} else {
				if before != "" {
					ErrorMessage(w, 400, fmt.Sprintf("before: %v", err))
					fmt.Printf("Could not parse before: %v\n", err)
					return
				}
			}

			after_t, err := time.Parse("2006-01-02T15:04:05Z07:00", after)
			after_query := ""
			if err == nil {
				after_query = `AND time >= ?`
				args = append(args, after_t)
			} else {
				if after != "" {
					ErrorMessage(w, 400, fmt.Sprintf("after: %v", err))
					fmt.Printf("Could not parse after: %v\n", err)
					return
				}
			}

			count_v, err := strconv.Atoi(count)
			if err != nil {
				count_v = 25
			}

			args = append(args, count_v)

			entries, err := database.FetchEntries(db, fmt.Sprintf(`WHERE server.value = ? 
					AND station.value = ?
					%v %v
					ORDER BY time %v
					LIMIT ?
					`, before_query, after_query, order), args...)

			if err != nil {
				if err == sql.ErrNoRows {
					ErrorMessage(w, 404, "No entries found")
					return
				}
				fmt.Printf("Could not fetch entries: %v\n", err)
				ErrorMessage(w, 500, "Could not fetch entries")
				return
			}

			w.Header().Set("Content-Type", "application/json")

			data, _ := json.Marshal(entries)
			w.Write(data)
		})
}

func StationRapidUpdatesRoute(db *sql.DB, brokers map[string]stations.Broker, r *mux.Router) {
	r.HandleFunc("/station/{server}/{station}/conditions/rapid/",
		func(w http.ResponseWriter, r *http.Request) {
			vars := mux.Vars(r)
			server := vars["server"]
			station := vars["station"]

			w.Header().Set("Cache-Control", "no-cache")

			broker, exists := brokers[server]
			if !exists {
				ErrorMessage(w, 404, "No station found")
				return
			}

			_, exists, err := database.LastStationInfoUpdate(db, server, station)
			if !exists {
				ErrorMessage(w, 404, "No station found")
				return
			}
			if err != nil {
				fmt.Printf("Could not fetch entries: %v\n", err)
				ErrorMessage(w, 500, "Could not fetch entries")
				return
			}

			updates := make(chan types.WeatherMessage)
			defer func() {
				broker.UnsubscribeRapidWeatherUpdates(station, updates)
				close(updates)
			}()

			err = broker.SubscribeRapidWeatherUpdates(station, updates)
			if err != nil {
				fmt.Printf("Could not start rapid weather updates: %v\n", err)
				ErrorMessage(w, 500, "Could not start rapid weather updates")
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(200)
			w.(http.Flusher).Flush()

			for {
				select {
				case message := <-updates:
					content, err := json.Marshal(message)
					if err != nil {
						fmt.Printf("Could not marshal message: %v\n", err)
						break
					}
					w.Write([]byte(fmt.Sprintf("data: %v\n\n", string(content))))
					w.(http.Flusher).Flush()
				case <-r.Context().Done():
					return
				}
			}
		})
}

func StationUpdatesRoute(db *sql.DB, brokers map[string]stations.Broker, r *mux.Router) {
	r.HandleFunc("/station/{server}/{station}/conditions/updates/",
		func(w http.ResponseWriter, r *http.Request) {
			vars := mux.Vars(r)
			server := vars["server"]
			station := vars["station"]

			w.Header().Set("Cache-Control", "no-cache")

			broker, exists := brokers[server]
			if !exists {
				ErrorMessage(w, 404, "No station found")
				return
			}

			_, exists, err := database.LastStationInfoUpdate(db, server, station)
			if !exists {
				ErrorMessage(w, 404, "No station found")
				return
			}
			if err != nil {
				fmt.Printf("Could not fetch entries: %v\n", err)
				ErrorMessage(w, 500, "Could not fetch entries")
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(200)
			w.(http.Flusher).Flush()

			updates := make(chan types.WeatherMessage)
			defer func() {
				broker.UnsubscribeWeatherUpdates(station, updates)
				close(updates)
			}()

			broker.SubscribeWeatherUpdates(station, updates)

			for {
				select {
				case message := <-updates:
					content, err := json.Marshal(message)
					if err != nil {
						fmt.Printf("Could not marshal message: %v\n", err)
						break
					}
					w.Write([]byte(fmt.Sprintf("data: %v\n\n", string(content))))
					w.(http.Flusher).Flush()
				case <-r.Context().Done():
					return
				}
			}
		})
}

func StationInfoRoute(db *sql.DB, r *mux.Router) {
	r.HandleFunc("/station/{server}/{station}/info/",
		func(w http.ResponseWriter, r *http.Request) {
			vars := mux.Vars(r)
			server := vars["server"]
			station := vars["station"]

			w.Header().Set("Cache-Control", "no-cache")

			info, exists, err := database.FetchStationInfo(db, server, station)
			if err != nil {
				fmt.Printf("Could not fetch entries: %v\n", err)
			}
			if !exists {
				ErrorMessage(w, 404, "No station found")
				return
			}
			if err != nil {
				ErrorMessage(w, 500, "Could not fetch entries")
				return
			}

			data, err := json.Marshal(info)
			if err != nil {
				ErrorMessage(w, 500, "Could not encode entry")
				fmt.Printf("Could not encode entry: %v\n", err)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(data)

		})
}
