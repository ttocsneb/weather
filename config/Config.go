package config

import (
	"github.com/BurntSushi/toml"
	"os"
)

type Config struct {
	Brokers map[string]string
	Id      string
}

func ParseConfig(path string) (Config, error) {
	var conf Config
	f, e := os.ReadFile(path)
	if e != nil {
		return conf, e
	}
	_, err := toml.Decode(string(f), &conf)
	if err != nil {
		return conf, err
	}
	return conf, nil
}
