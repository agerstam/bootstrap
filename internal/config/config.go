package config

type Config struct {
	Bootstrap struct {
		TokenId string `yaml:"token-id"`
		Version string `yaml:"version"`
	} `yaml:"bootstrap"`
}
