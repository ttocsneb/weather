package main

import (
	"fmt"

	"os"

	"github.com/ttocsneb/weather/config"
	"github.com/ttocsneb/weather/server"
	"github.com/ttocsneb/weather/stations"

	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	args := os.Args
	path := "config.toml"
	if len(args) > 1 {
		path = args[1]
	}

	conf, err := config.ParseConfig(path)
	if err != nil {
		fmt.Printf("Could not load “%v”: %v\n", path, err)
		return
	}

	db, err := sql.Open("sqlite3", "db.sqlite3")
	if err != nil {
		panic(err)
	}
	err = db.Ping()
	if err != nil {
		panic(err)
	}

	brokers := make(map[string]stations.Broker)

	for broker, server := range conf.Brokers {
		server, err := stations.NewBroker(db, conf.Id, broker, server)
		if err != nil {
			panic(err)
		}
		brokers[broker] = server
		defer brokers[broker].Client.Disconnect(500)
	}

	fmt.Println("Started Server")

	server.Serve(db, brokers)
}
