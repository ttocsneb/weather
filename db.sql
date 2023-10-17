CREATE TABLE lookup_strings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    value TEXT 
);

CREATE TABLE sensor_value (
    entry_id INTEGER,
    name_id INTEGER,
    sensor_number INTEGER,
    unit_id INTEGER,
    value FLOAT,
    PRIMARY KEY (entry_id, name_id, sensor_number)
);

CREATE TABLE weather_entry (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    station_id INTEGER,
    server_id INTEGER,
    time DATETIME
);

CREATE TABLE station (
    server_id INTEGER,
    station_id INTEGER,
    make_id INTEGER,
    model_id INTEGER,
    software_id INTEGER,
    version_id INTEGER,
    latitude FLOAT,
    longitude FLOAT,
    elevation FLOAT,
    district_id INTEGER,
    city_id INTEGER,
    region_id INTEGER,
    country_id INTEGER,
    rapid_weather BOOLEAN,
    updated DATETIME,
    PRIMARY KEY (station_id, server_id)
);
