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

func LoadSettings(filePath string) (*AppConfig, error) {
	// Parse the YML file
	var cfg AppConfig

	// Open the YML file
	file, err := os.Open(filePath)
	if err != nil {
		return &cfg, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return &cfg, fmt.Errorf("failed to parse YAML file: %w", err)
	}
	return &cfg, nil
}

func (cfg *AppConfig) Validate() error {

	if cfg.LUKS.VolumePath == "" {
		cfg.LUKS.VolumePath = "udm-luks.img" // default value
	}
	if cfg.LUKS.MapperName == "" {
		cfg.LUKS.MapperName = "udm-luks" // default value
	}
	if cfg.LUKS.MountPoint == "" {
		cfg.LUKS.MountPoint = "mnt/udm-luks" // default value
	}
	if cfg.LUKS.PasswordLength == 0 {
		cfg.LUKS.PasswordLength = 20 // default value
	}
	if cfg.LUKS.Size == 0 {
		cfg.LUKS.Size = 10 // default value
	}
	if cfg.LUKS.User == "" {
		cfg.LUKS.User = "root" // default value
	}
	if cfg.LUKS.Group == "" {
		cfg.LUKS.Group = "root" // default value
	}
	return nil
}
