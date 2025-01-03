package config

import (
	"bootstrap/internal/luks"
)

type Command struct {
	CommandName string // Command to execute
	Config      string // Path to config YAML
	Bootstrap   string // Path to bootstrap YAML
	Keyfile     string // Path to keyfile
}

type BootstrapToken struct {
	Bootstrap struct {
		TokenId string `yaml:"token-id"`
		Version string `yaml:"version"`
	} `yaml:"bootstrap"`
}

type AppConfig struct {
	Cmd     Command   // Command to execute
	Verbose *bool     // Verbose logging
	LUKS    luks.LUKS `yaml:"luks"` // LUKS configuration
}
