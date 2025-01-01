package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func LoadConfig(filePath string) (*Config, error) {
	// Open the YML file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Parse the YML file
	var config Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file: %w", err)
	}

	return &config, nil
}

func (cfg *Config) Validate() error {
	if cfg.Bootstrap.TokenId == "" {
		return fmt.Errorf("bootstrap.token-id is required")
	}
	if cfg.Bootstrap.Version == "" {
		return fmt.Errorf("bootstrap.version is required")
	}
	return nil
}
