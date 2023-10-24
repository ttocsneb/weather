package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ttocsneb/weather/types"
	"github.com/ttocsneb/weather/util"
)

func GenStringJoins(table string, properties ...string) string {
	joins := ""
	for _, property := range properties {
		joins += fmt.Sprintf("JOIN lookup_strings %v ON %v.%v_id = %v.id\n",
			property, table, property, property,
		)
	}
	return joins
}

func MakeUnique(strs []string) []string {
	unique := []string{}
	for _, str := range strs {
		if _, exists := util.IndexOf(unique, &str); !exists {
			unique = append(unique, str)
		}
	}
	return unique
}

func FetchLookupStrings(db *sql.DB, strs []string) (map[string]int, error) {
	strs = MakeUnique(strs)
	query := "SELECT * FROM lookup_strings WHERE value IN ("
	placeholders := make([]string, len(strs))
	for i := range strs {
		placeholders[i] = "?"
	}
	query += fmt.Sprintf("%s);", strings.Join(placeholders, ", "))

	args := make([]interface{}, len(strs))
	for i, str := range strs {
		args[i] = str
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return make(map[string]int), err
	}
	defer rows.Close()

	strings := make(map[string]int)

	for rows.Next() {
		var id int
		var value string
		if err := rows.Scan(&id, &value); err != nil {
			return make(map[string]int), err
		}

		strings[value] = id
	}

	return strings, nil
}

func InsertLookupStrings(db *sql.DB, strs []string) error {
	strs = MakeUnique(strs)
	query := "INSERT INTO lookup_strings (value) VALUES "
	placeholders := make([]string, len(strs))
	for i := range strs {
		placeholders[i] = "(?)"
	}
	query += fmt.Sprintf("%s;", strings.Join(placeholders, ", "))

	args := make([]interface{}, len(strs))
	for i, str := range strs {
		args[i] = str
	}

	_, err := db.Exec(query, args...)
	return err
}

func GetOrInsertLookupStrings(db *sql.DB, strs []string) (map[string]int, error) {
	found, err := FetchLookupStrings(db, strs)
	if err != nil {
		return make(map[string]int), err
	}

	to_create := []string{}
	for _, str := range strs {
		_, exists := found[str]
		if !exists {
			to_create = append(to_create, str)
		}
	}

	if len(to_create) > 0 {
		if err := InsertLookupStrings(db, to_create); err != nil {
			return make(map[string]int), err
		}
		created, err := FetchLookupStrings(db, to_create)
		if err != nil {
			return make(map[string]int), err
		}

		for key, id := range created {
			found[key] = id
		}
	}

	return found, nil
}

func InsertWeatherEntry(db *sql.DB, entry types.WeatherEntry) (int64, error) {
	string_list := make([]string, 2)
	string_list[0] = entry.Station
	string_list[1] = entry.Server
	for key, sensors := range entry.Sensors {
		string_list = append(string_list, key)
		for _, sensor := range sensors {
			string_list = append(string_list, sensor.Unit)
		}
	}

	lookup, err := GetOrInsertLookupStrings(db, string_list)
	if err != nil {
		return 0, err
	}

	query := `INSERT INTO weather_entry (station_id, server_id, time) 
					VALUES (?, ?, ?);`

	result, err := db.Exec(query,
		lookup[entry.Station],
		lookup[entry.Server],
		entry.Time,
	)
	if err != nil {
		return 0, err
	}
	entry_id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	query = `INSERT INTO sensor_value 
					(entry_id, name_id, sensor_number, unit_id, value) 
					VALUES `

	opts := []string{}
	args := []interface{}{}

	for name, sensors := range entry.Sensors {
		for number, sensor := range sensors {
			opts = append(opts, "(?, ?, ?, ?, ?)")

			args = append(args, entry_id)
			args = append(args, lookup[name])
			args = append(args, number)
			args = append(args, lookup[sensor.Unit])
			args = append(args, sensor.Value)

		}
	}

	query += fmt.Sprintf("%v;", strings.Join(opts, ", "))

	_, err = db.Exec(query, args...)

	return entry_id, err
}

func fetchSensorsFromEntry(db *sql.DB, id int) (map[string][]types.SensorValue, error) {
	query := fmt.Sprintf(`SELECT 
		sensor_number, name.value, unit.value, sensor_value.value 
		FROM sensor_value 
		%v
		WHERE entry_id = ? 
		ORDER BY name_id ASC, unit_id ASC;`,
		GenStringJoins("sensor_value", "name", "unit"),
	)

	rows, err := db.Query(query, id)
	if err != nil {
		return make(map[string][]types.SensorValue), err
	}

	sensors := make(map[string][]types.SensorValue)

	for rows.Next() {
		var sensor_number int
		var name string
		var unit string
		var value float64
		if err := rows.Scan(&sensor_number, &name, &unit, &value); err != nil {
			return make(map[string][]types.SensorValue), err
		}

		sensor, exists := sensors[name]
		if !exists {
			sensor = []types.SensorValue{}
		}

		sensors[name] = append(sensor, types.SensorValue{
			Unit:  unit,
			Value: value,
		})
	}
	return sensors, nil
}

func FetchEntry(db *sql.DB, condition string, args ...any) (types.WeatherEntry, error) {
	query := fmt.Sprintf(`SELECT 	weather_entry.id, 
											station.value, 
											server.value, 
											time FROM weather_entry
			%v %v
			LIMIT 1;`,
		GenStringJoins("weather_entry", "station", "server"),
		condition)

	row := db.QueryRow(query, args...)
	var id int
	var station string
	var server string
	var time time.Time
	err := row.Scan(&id, &station, &server, &time)
	if err != nil {
		return types.WeatherEntry{}, err
	}

	sensors, err := fetchSensorsFromEntry(db, id)
	if err != nil {
		return types.WeatherEntry{}, err
	}

	return types.WeatherEntry{
		Time:    time,
		Station: station,
		Server:  server,
		Sensors: sensors,
	}, nil
}

func FetchEntries(db *sql.DB, condition string, args ...any) ([]types.WeatherEntry, error) {
	query := fmt.Sprintf(`SELECT 	weather_entry.id, 
											station.value, 
											server.value, 
											time FROM weather_entry
					%v %v;`,
		GenStringJoins("weather_entry", "station", "server"),
		condition)

	rows, err := db.Query(query, args...)
	if err != nil {
		return []types.WeatherEntry{}, err
	}

	entries := []types.WeatherEntry{}

	for rows.Next() {
		var id int
		var station string
		var server string
		var time time.Time
		err := rows.Scan(&id, &station, &server, &time)
		if err != nil {
			return []types.WeatherEntry{}, err
		}

		sensors, err := fetchSensorsFromEntry(db, id)
		if err != nil {
			return []types.WeatherEntry{}, err
		}

		entries = append(entries, types.WeatherEntry{
			Station: station,
			Server:  server,
			Time:    time,
			Sensors: sensors,
		})
	}

	return entries, nil
}

func LastStationInfoUpdate(db *sql.DB, server string, station string) (time.Time, bool, error) {
	query := fmt.Sprintf(`SELECT updated FROM station 
			%v
			WHERE server.value = ? AND station.value = ?`,
		GenStringJoins("station", "server", "station"))

	row := db.QueryRow(query, server, station)

	var updated time.Time
	err := row.Scan(&updated)
	if err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return updated, true, nil
}

func FetchStationInfo(db *sql.DB, server string, station string) (types.StationEntry, bool, error) {
	query := fmt.Sprintf(`SELECT 
			server.value, station.value, make.value, model.value, software.value,
			version.value, latitude, longitude, elevation, district.value, 
			city.value, region.value, country.value, rapid_weather, updated 
		FROM station 
		%v
		WHERE server.value = ? AND station.value = ? 
		LIMIT 1;`,
		GenStringJoins("station", "make", "model", "software", "version",
			"district", "city", "region", "country", "server", "station"))

	row := db.QueryRow(query, server, station)

	var server_val string
	var station_val string
	var make_val string
	var model string
	var software string
	var version string
	var latitude float64
	var longitude float64
	var elevation float64
	var district string
	var city string
	var region string
	var country string
	var rapid_weather bool
	var updated time.Time

	err := row.Scan(&server_val, &station_val, &make_val, &model, &software,
		&version, &latitude, &longitude, &elevation, &district, &city, &region,
		&country, &rapid_weather, &updated)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.StationEntry{}, true, err
		}
		return types.StationEntry{}, false, err
	}

	return types.StationEntry{
		Server:       server,
		Station:      station,
		Make:         make_val,
		Model:        model,
		Software:     software,
		Version:      version,
		Latitude:     latitude,
		Longitude:    longitude,
		Elevation:    elevation,
		District:     district,
		City:         city,
		Region:       region,
		Country:      country,
		RapidWeather: rapid_weather,
		Updated:      updated,
	}, true, nil

}

func FetchStationInfos(db *sql.DB, stations []types.StationKey) ([]types.StationEntry, error) {
	conditions := make([]string, len(stations))
	args := make([]interface{}, len(stations)*2)

	for i, station := range stations {
		conditions[i] = "(server.value = ? AND station.value = ?)"
		args[i*2] = station.Server
		args[i*2+1] = station.Station
	}

	query := fmt.Sprintf(`SELECT 
			server.value, station.value, make.value, model.value, software.value,
			version.value, latitude, longitude, elevation, district.value, 
			city.value, region.value, country.value, rapid_weather, updated 
		FROM station 
		%v
		WHERE %v;`,
		GenStringJoins("station", "make", "model", "software", "version",
			"district", "city", "region", "country", "server", "station"),
		strings.Join(conditions, " OR "))

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}

	output := []types.StationEntry{}

	for rows.Next() {
		var server_val string
		var station_val string
		var make_val string
		var model string
		var software string
		var version string
		var latitude float64
		var longitude float64
		var elevation float64
		var district string
		var city string
		var region string
		var country string
		var rapid_weather bool
		var updated time.Time

		err := rows.Scan(&server_val, &station_val, &make_val, &model, &software,
			&version, &latitude, &longitude, &elevation, &district, &city, &region,
			&country, &rapid_weather, &updated)
		if err != nil {
			return nil, err
		}

		output = append(output, types.StationEntry{
			Server:       server_val,
			Station:      station_val,
			Make:         make_val,
			Model:        model,
			Software:     software,
			Version:      version,
			Latitude:     latitude,
			Longitude:    longitude,
			Elevation:    elevation,
			District:     district,
			City:         city,
			Region:       region,
			Country:      country,
			RapidWeather: rapid_weather,
			Updated:      updated,
		})
	}

	return output, nil
}

func UpdateStationInfo(db *sql.DB, entry types.StationEntry) error {
	lookup, err := GetOrInsertLookupStrings(db, []string{
		entry.Server,
		entry.Station,
		entry.Make,
		entry.Model,
		entry.Software,
		entry.Version,
		entry.District,
		entry.City,
		entry.Region,
		entry.Country,
	})
	query := `SELECT COUNT(station_id) FROM station
		WHERE server_id = ? AND station_id = ?;`

	row := db.QueryRow(query, lookup[entry.Server], lookup[entry.Station])

	var count int
	err = row.Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		query = `UPDATE station SET 
			make_id = ?,
			model_id = ?,
			software_id = ?,
			version_id = ?,
			latitude = ?,
			longitude = ?,
			elevation = ?,
			district_id = ?,
			city_id = ?,
			region_id = ?,
			country_id = ?,
			rapid_weather = ?,
			updated = ?
			WHERE server_id = ? AND station_id = ?;`
	} else {
		query = `INSERT INTO station (
			make_id,
			model_id,
			software_id,
			version_id,
			latitude,
			longitude,
			elevation,
			district_id,
			city_id,
			region_id,
			country_id,
			rapid_weather,
			updated,
			server_id,
			station_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
	}

	_, err = db.Exec(query, lookup[entry.Make], lookup[entry.Model],
		lookup[entry.Software], lookup[entry.Version], entry.Latitude,
		entry.Longitude, entry.Elevation, lookup[entry.District],
		lookup[entry.City], lookup[entry.Region], lookup[entry.Country],
		entry.RapidWeather, entry.Updated, lookup[entry.Server],
		lookup[entry.Station])
	return err
}

func QueryStationInfos(db *sql.DB, condition string, args ...interface{}) ([]types.StationEntry, error) {
	query := fmt.Sprintf(`SELECT 
			server.value, 
			station.value,
			make.value,
			model.value,
			software.value,
			version.value,
			latitude,
			longitude,
			elevation,
			district.value,
			city.value,
			region.value,
			country.value,
			rapid_weather,
			updated 
		FROM station 
		%v %v;`,
		GenStringJoins("station", "make", "model", "software", "version",
			"district", "city", "region", "country", "server", "station"),
		condition)

	result := []types.StationEntry{}

	rows, err := db.Query(query, args...)
	if err != nil {
		return result, err
	}

	for rows.Next() {
		var server_val string
		var station_val string
		var make_val string
		var model string
		var software string
		var version string
		var latitude float64
		var longitude float64
		var elevation float64
		var district string
		var city string
		var region string
		var country string
		var rapid_weather bool
		var updated time.Time

		err := rows.Scan(&server_val, &station_val, &make_val, &model,
			&software, &version, &latitude, &longitude, &elevation, &district,
			&city, &region, &country, &rapid_weather, &updated)
		if err != nil {
			return result, err
		}

		info := types.StationEntry{
			Server:       server_val,
			Station:      station_val,
			Make:         make_val,
			Model:        model,
			Software:     software,
			Version:      version,
			Latitude:     latitude,
			Longitude:    longitude,
			Elevation:    elevation,
			District:     district,
			City:         city,
			Region:       region,
			Country:      country,
			RapidWeather: rapid_weather,
			Updated:      updated,
		}
		result = append(result, info)
	}

	return result, nil
}
