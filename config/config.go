package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	LimbusFolder           string `json:"limbus_folder"`
	CurrentVersion         string `json:"current_version"`
	CurrentBotVersion      string `json:"current_bot_version"`
	CurrentLimboniaVersion string `json:"current_limbonia_version"`
}

var config *Config

func Get() *Config {
	return config
}

func Init() {
	if _, err := os.Stat("config.json"); os.IsNotExist(err) {
		config = &Config{
			LimbusFolder:      "",
			CurrentVersion:    "",
			CurrentBotVersion: "",
		}
		Save()
		return
	}
	Load()
}

func Load() {
	file, err := os.ReadFile("config.json")
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(file, &config)

	if err != nil {
		panic(err)
	}
}

func Save() {
	file, err := os.Create("config.json")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	data, err := json.Marshal(config)
	if err != nil {
		panic(err)
	}

	_, err = file.Write(data)
	if err != nil {
		panic(err)
	}
}
