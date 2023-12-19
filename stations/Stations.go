package stations

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
	"github.com/ttocsneb/weather/database"
	"github.com/ttocsneb/weather/types"
	"github.com/ttocsneb/weather/util"
)

type ChanMux struct {
	updates []chan types.WeatherMessage
	done    chan interface{}
}

func newChanMux(broker *Broker, station string, listener chan types.WeatherMessage, on_done func(*ChanMux)) (*ChanMux, error) {
	self := new(ChanMux)
	self.updates = []chan types.WeatherMessage{listener}
	self.done = make(chan interface{})

	fmt.Println("Creating rapid-weather listener")

	subscription := fmt.Sprintf("/station/rapid-weather/%v", station)
	request := fmt.Sprintf("/station/request/%v", station)

	err := WaitOrErr(broker.Client.Subscribe(subscription, 1, func(client mqtt.Client, msg mqtt.Message) {
		var payload types.WeatherMessage
		err := json.Unmarshal(msg.Payload(), &payload)
		if err != nil {
			fmt.Printf("Could not parse rapid-weather message: %v\n", err)
			return
		}

		message := types.WeatherMessage{
			Time:    payload.Time,
			ID:      payload.ID,
			Sensors: make(map[string][]types.SensorValue),
		}

		for sensor, values := range payload.Sensors {
			message.Sensors[sensor] = make([]types.SensorValue, len(values))
			for i, value := range values {
				val, unit := util.SensorToMetric(value.Value, value.Unit, sensor)
				message.Sensors[sensor][i] = types.SensorValue{
					Unit:  unit,
					Value: val,
				}
			}
		}

		for _, ch := range self.updates {
			ch <- message
		}
	}))
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(types.RequestMessage{Action: "rapid-weather"})
	if err != nil {
		return nil, err
	}
	err = WaitOrErr(broker.Client.Publish(request, 1, false, payload))
	if err != nil {
		return nil, err
	}
	keepAlive := func() {
		timeout := time.After(time.Second * 50)
		for true {
			select {
			case <-self.done:
				err := WaitOrErr(broker.Client.Unsubscribe(subscription))
				if err != nil {
					fmt.Printf("Could not Unsubscribe from rapid-weather updates: %v\n", err)
				}
				fmt.Println("Closing rapid-weather listener")
				on_done(self)
				return
			case <-timeout:
				err := WaitOrErr(broker.Client.Publish(request, 1, false, payload))
				if err != nil {
					fmt.Printf("Could not send rapid-weather request: %v\n", err)
					break
				}
				timeout = time.After(time.Second * 50)
			}
		}
	}

	go keepAlive()

	return self, nil
}

func (self *ChanMux) addListener(listener chan types.WeatherMessage) {
	self.updates = append(self.updates, listener)
	fmt.Printf("There are %v listeners", len(self.updates))
}

func (self *ChanMux) delListener(listener chan types.WeatherMessage) bool {
	for i, l := range self.updates {
		if l == listener {
			self.updates = append(self.updates[:i], self.updates[i+1:]...)
			if len(self.updates) == 0 {
				self.done <- true
			}
			return true
		}
	}
	return false
}

type Broker struct {
	Client         mqtt.Client
	Broker         string
	db             *sql.DB
	rapidUpdates   map[string]*ChanMux
	stationUpdates map[string][]chan types.WeatherMessage
	updates        []chan types.WeatherMessage
}

func WaitOrErr(fut mqtt.Token) error {
	fut.Wait()
	return fut.Error()
}

func (self *Broker) WeatherListener() mqtt.MessageHandler {
	return func(client mqtt.Client, msg mqtt.Message) {
		var payload types.WeatherMessage
		if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
			fmt.Printf("Unable to parse message: %v\n", err)
			return
		}

		message := types.WeatherMessage{
			Time:    payload.Time,
			ID:      payload.ID,
			Sensors: make(map[string][]types.SensorValue),
		}

		for sensor, values := range payload.Sensors {
			message.Sensors[sensor] = make([]types.SensorValue, len(values))
			for i, value := range values {
				val, unit := util.SensorToMetric(value.Value, value.Unit, sensor)
				message.Sensors[sensor][i] = types.SensorValue{
					Unit:  unit,
					Value: val,
				}
			}
		}

		hooks, exists := self.stationUpdates[payload.ID]
		if exists {
			for _, hook := range hooks {
				hook <- message
			}
		}

		_, err := database.InsertWeatherEntry(self.db, message.ToEntry(self.Broker))
		if err != nil {
			fmt.Printf("Unable to save message to db: %v\n", err)
			return
		}
		fmt.Printf("Received Message from %v\n", self.Broker)

		t, exists, err := database.LastStationInfoUpdate(self.db, self.Broker, payload.ID)
		if err != nil {
			fmt.Printf("Unable to check station from db: %v\n", err)
			return
		}
		if !exists || time.Now().Sub(t) > time.Hour*24 {
			entry, err := self.FetchStationInfo(payload.ID)
			if err != nil {
				fmt.Printf("Unable to fetch station info: %v\n", err)
				return
			}
			fmt.Printf("Fetched station info for %v - %v\n", entry.Server, entry.Station)
		}

	}
}

func (self *Broker) FetchStationInfo(station string) (types.StationEntry, error) {
	type message struct {
		msg types.StationMessage
		err error
	}
	on_recv := make(chan message)
	subscription := fmt.Sprintf("/station/info/%v", station)
	request := fmt.Sprintf("/station/request/%v", station)
	err := WaitOrErr(self.Client.Subscribe(subscription, 1, func(client mqtt.Client, msg mqtt.Message) {
		var payload types.StationMessage
		err := json.Unmarshal(msg.Payload(), &payload)
		on_recv <- message{
			msg: payload,
			err: err,
		}
	}))
	if err != nil {
		return types.StationEntry{}, err
	}
	time.Sleep(time.Millisecond * 250)
	payload, err := json.Marshal(types.RequestMessage{Action: "info"})
	if err != nil {
		return types.StationEntry{}, err
	}
	err = WaitOrErr(self.Client.Publish(request, 1, false, payload))
	if err != nil {
		return types.StationEntry{}, err
	}
	recv := <-on_recv
	if recv.err != nil {
		return types.StationEntry{}, err
	}
	info := recv.msg.ToEntry(self.Broker, station, time.Now())
	err = database.UpdateStationInfo(self.db, info)
	wait_err := WaitOrErr(self.Client.Unsubscribe(subscription))
	if wait_err != nil {
		return types.StationEntry{}, err
	}
	if err != nil {
		return types.StationEntry{}, err
	}
	return info, nil
}

func (self *Broker) SubscribeRapidWeatherUpdates(station string, weather chan types.WeatherMessage) error {
	mux, exists := self.rapidUpdates[station]
	if !exists {
		mux, err := newChanMux(self, station, weather, func(cm *ChanMux) {
			delete(self.rapidUpdates, station)
		})
		if err != nil {
			return err
		}
		self.rapidUpdates[station] = mux
		return nil
	}
	mux.addListener(weather)

	return nil
}
func (self *Broker) UnsubscribeRapidWeatherUpdates(station string, weather chan types.WeatherMessage) bool {
	mux, exists := self.rapidUpdates[station]
	if !exists {
		return false
	}
	return mux.delListener(weather)
}

func (self *Broker) SubscribeWeatherUpdates(station string, weather chan types.WeatherMessage) {
	list, exists := self.stationUpdates[station]
	if !exists {
		list = []chan types.WeatherMessage{}
	}

	list = append(list, weather)
	self.stationUpdates[station] = list
}
func (self *Broker) UnsubscribeWeatherUpdates(station string, weather chan types.WeatherMessage) bool {
	list, exists := self.stationUpdates[station]
	if !exists {
		return false
	}
	for i, val := range list {
		if val == weather {
			list = append(list[:i], list[i+1:]...)
			if len(list) == 0 {
				delete(self.stationUpdates, station)
			} else {
				self.stationUpdates[station] = list
			}
			return true
		}
	}
	return false
}

func (self *Broker) SubscribeAllWeatherUpdates(weather chan types.WeatherMessage) {
	self.updates = append(self.updates, weather)
}
func (self *Broker) UnsubscribeAllWeatherUpdates(weather chan types.WeatherMessage) bool {
	for i, val := range self.updates {
		if val == weather {
			self.updates = append(self.updates[:i], self.updates[i+1:]...)
			return true
		}
	}
	return false
}

func NewBroker(db *sql.DB, id string, broker string, server string) (Broker, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(server)
	opts.SetClientID(id)
	opts.SetOrderMatters(false)
	client := mqtt.NewClient(opts)

	if err := WaitOrErr(client.Connect()); err != nil {
		return Broker{}, err
	}

	self := Broker{
		Client:         client,
		Broker:         broker,
		db:             db,
		rapidUpdates:   make(map[string]*ChanMux),
		stationUpdates: make(map[string][]chan types.WeatherMessage),
		updates:        []chan types.WeatherMessage{},
	}

	if err := WaitOrErr(client.Subscribe("/station/weather/+", 0,
		self.WeatherListener())); err != nil {
		return Broker{}, err
	}

	return self, nil
}
