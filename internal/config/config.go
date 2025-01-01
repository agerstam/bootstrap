package config

import (
	"bootstrap/internal/luks"
)

type BootstrapToken struct {
	Bootstrap struct {
		TokenId string `yaml:"token-id"`
		Version string `yaml:"version"`
	} `yaml:"bootstrap"`
}

type AppConfig struct {
	Verbose   bool          // Verbose logging
	luks.LUKS `yaml:"luks"` // LUKS configuration
	/*
		LUKS    struct {
			VolumePath     string `yaml:"volumePath"`
			MapperName     string `yaml:"mapperName"`
			MountPoint     string `yaml:"mountpoint"`
			PasswordLength int    `yaml:"passwordLength"`
			Password       string `yaml:"-"`
			Size           int    `yaml:"size"`
			UseTPM         bool   `yaml:"useTPM"`
		} `yaml:"luks"`
	*/
}

// LoadConfig reads the YAML configuration file and returns a BootstrapToken
