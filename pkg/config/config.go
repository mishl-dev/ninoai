package config

import (
	"os"
	"gopkg.in/yaml.v3"
)

type Config struct {
	EmbeddingAPIURL string `yaml:"embedding_api_url"`
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
