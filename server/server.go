package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/ttocsneb/weather/stations"
)

type errorMsg struct {
	Message string `json:"message"`
}

func ErrorMessage(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	data, _ := json.Marshal(errorMsg{Message: message})
	w.Write(data)
}

func Serve(db *sql.DB, brokers map[string]stations.Broker) {
	r := mux.NewRouter()

	StationConditionsRoute(db, r)
	StationHistoryRoute(db, r)
	StationRapidUpdatesRoute(db, brokers, r)
	StationUpdatesRoute(db, brokers, r)
	StationInfoRoute(db, r)

	fmt.Println("Starting server..")

	err := http.ListenAndServe(":8080", r)
	panic(err)
}
