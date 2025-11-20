package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ModelSettings struct {
		Temperature float64 `yaml:"temperature"`
		TopP        float64 `yaml:"top_p"`
	} `yaml:"model_settings"`
	Delays struct {
		MessageProcessing float64 `yaml:"message_processing"`
	} `yaml:"delays"`
}

func defaultConfig() *Config {
	cfg := &Config{}
	cfg.ModelSettings.Temperature = 0.7
	cfg.ModelSettings.TopP = 0.9
	cfg.Delays.MessageProcessing = 1.5
	return cfg
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("config.yml not found, using default settings.")
			return defaultConfig(), nil
		}
		return nil, err
	}

	config := &Config{}
	err = yaml.Unmarshal(file, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
