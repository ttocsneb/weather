package types

import (
	"fmt"
	"time"
)

type StationKey struct {
	Server  string `json:"server"`
	Station string `json:"station"`
}

type SensorValue struct {
	Unit  string  `json:"unit"`
	Value float64 `json:"value"`
}

type WeatherMessage struct {
	Time    time.Time                `json:"time"`
	ID      string                   `json:"id"`
	Sensors map[string][]SensorValue `json:"sensors"`
}

type WeatherEntry struct {
	Station string                   `json:"station"`
	Server  string                   `json:"server"`
	Time    time.Time                `json:"time"`
	Sensors map[string][]SensorValue `json:"sensors"`
}

func (self *WeatherMessage) ToEntry(server string) WeatherEntry {
	return WeatherEntry{
		Station: self.ID,
		Server:  server,
		Time:    self.Time,
		Sensors: self.Sensors,
	}
}

func (self *WeatherEntry) MapId() string {
	return MapId(self.Server, self.Station)
}

type StationMessage struct {
	Make         string  `json:"make"`
	Model        string  `json:"model"`
	Software     string  `json:"software"`
	Version      string  `json:"version"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	Elevation    float64 `json:"elevation"`
	District     string  `json:"district"`
	City         string  `json:"city"`
	Region       string  `json:"region"`
	Country      string  `json:"country"`
	RapidWeather bool    `json:"rapid-weather"`
}

type StationEntry struct {
	Server       string    `json:"server"`
	Station      string    `json:"Station"`
	Make         string    `json:"make"`
	Model        string    `json:"model"`
	Software     string    `json:"software"`
	Version      string    `json:"version"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	Elevation    float64   `json:"elevation"`
	District     string    `json:"district"`
	City         string    `json:"city"`
	Region       string    `json:"region"`
	Country      string    `json:"country"`
	RapidWeather bool      `json:"rapidWeather"`
	Updated      time.Time `json:"updated"`
}

func (self *StationEntry) MapId() string {
	return MapId(self.Server, self.Station)
}

func MapId(server string, station string) string {
	return fmt.Sprintf("%v-%v", server, station)
}

func (self StationMessage) ToEntry(server string, station string, updated time.Time) StationEntry {
	return StationEntry{
		Server:       server,
		Station:      station,
		Make:         self.Make,
		Model:        self.Model,
		Software:     self.Software,
		Version:      self.Version,
		Latitude:     self.Latitude,
		Longitude:    self.Longitude,
		Elevation:    self.Elevation,
		District:     self.District,
		City:         self.City,
		Region:       self.Region,
		Country:      self.Country,
		RapidWeather: self.RapidWeather,
		Updated:      updated,
	}
}

type RequestMessage struct {
	Action string `json:"action"`
}
