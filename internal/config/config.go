package Config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Channel struct {
	ID string `json:"id"`
	SaveTo string `json:"save_to"`
}

type Config struct {
	LogLevel string `json:"log_level"`
	Channels []Channel `json:"channels"`
	APIKey string `json:"APIKey"`
	Python string `json:"python"`
	QueryInterval int `json:"query_interval"`
}

func ReadConfig(file string) Config {
	var config Config
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		fmt.Println(err.Error())
	}
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	if err != nil {
		panic(err)
	}
	return config
}
