package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func LoadBootstrap(filePath string) (*BootstrapToken, error) {
	// Open the YML file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Parse the YML file
	var token BootstrapToken
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file: %w", err)
	}
	return &token, nil
}

func (cfg *BootstrapToken) Validate() error {
	if cfg.Bootstrap.TokenId == "" {
		return fmt.Errorf("bootstrap.token-id is required")
	}
	if cfg.Bootstrap.Version == "" {
		return fmt.Errorf("bootstrap.version is required")
	}
	return nil
}

func LoadSettings(filePath string) (*AppSettings, error) {
	// Open the YML file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Parse the YML file
	var settings AppSettings
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&settings); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file: %w", err)
	}
	return &settings, nil
}

func (cfg *AppSettings) Validate() error {

	if cfg.Settings.VolumePath == "" {
		cfg.Settings.VolumePath = "udm-luks.img" // default value
	}
	if cfg.Settings.MapperName == "" {
		cfg.Settings.MapperName = "udm-luks" // default value
	}
	if cfg.Settings.MountPoint == "" {
		cfg.Settings.MountPoint = "mnt/udm-luks" // default value
	}
	if cfg.Settings.PasswordLength == 0 {
		cfg.Settings.PasswordLength = 20 // default value
	}
	if cfg.Settings.Size == 0 {
		cfg.Settings.Size = 10 // default value
	}
	return nil
}
