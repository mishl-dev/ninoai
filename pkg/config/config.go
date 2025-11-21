package config

import (
	"os"
	"gopkg.in/yaml.v3"
)

type Config struct {
	ModelSettings   struct {
		Temperature float64 `yaml:"temperature"`
		TopP        float64 `yaml:"top_p"`
	} `yaml:"model_settings"`
	Delays struct {
		MessageProcessing float64 `yaml:"message_processing"`
	} `yaml:"delays"`
}

func LoadConfig(path string) (*Config, error) {
	config := &Config{}

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		// Set default values
		config.ModelSettings.Temperature = 0.4
		config.ModelSettings.TopP = 1
		config.Delays.MessageProcessing = 0.5
		return config, nil
	}

	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(file, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
