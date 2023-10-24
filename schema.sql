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
    PRIMARY KEY (entry_id, name_id, sensor_number),
    CONSTRAINT FK_entry FOREIGN KEY (entry_id) REFERENCES weather_entry(id),
    CONSTRAINT FK_name FOREIGN KEY (name_id) REFERENCES lookup_strings(id),
    CONSTRAINT FK_unit FOREIGN KEY (unit_id) REFERENCES weather_entry(id)
);

CREATE TABLE weather_entry (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    station_id INTEGER,
    server_id INTEGER,
    time DATETIME,
    CONSTRAINT FK_station FOREIGN KEY (station_id) REFERENCES lookup_strings(id),
    CONSTRAINT FK_server FOREIGN KEY (server_id) REFERENCES lookup_strings(id)
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
    PRIMARY KEY (station_id, server_id),
    CONSTRAINT FK_station FOREIGN KEY (station_id) REFERENCES lookup_strings(id),
    CONSTRAINT FK_server FOREIGN KEY (server_id) REFERENCES lookup_strings(id),
    CONSTRAINT FK_make FOREIGN KEY (make_id) REFERENCES lookup_strings(id),
    CONSTRAINT FK_model FOREIGN KEY (model_id) REFERENCES lookup_strings(id),
    CONSTRAINT FK_software FOREIGN KEY (software_id) REFERENCES lookup_strings(id),
    CONSTRAINT FK_version FOREIGN KEY (version_id) REFERENCES lookup_strings(id),
    CONSTRAINT FK_district FOREIGN KEY (district_id) REFERENCES lookup_strings(id),
    CONSTRAINT FK_city FOREIGN KEY (city_id) REFERENCES lookup_strings(id),
    CONSTRAINT FK_region FOREIGN KEY (region_id) REFERENCES lookup_strings(id),
    CONSTRAINT FK_country FOREIGN KEY (country_id) REFERENCES lookup_strings(id)
);
