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
	BootstrapFile *string   // Path to the bootstrap file
	Deauthorize   *bool     // Authorize or Deauthorize the node
	Verbose       *bool     // Verbose logging
	LUKS          luks.LUKS `yaml:"luks"` // LUKS configuration
}
