package config

type BootstrapToken struct {
	Bootstrap struct {
		TokenId string `yaml:"token-id"`
		Version string `yaml:"version"`
	} `yaml:"bootstrap"`
}

type AppSettings struct {
	Settings struct {
		VolumePath     string `yaml:"volumePath"`
		MapperName     string `yaml:"mapperName"`
		MountPoint     string `yaml:"mountpoint"`
		PasswordLength int    `yaml:"passwordLength"`
		Size           int    `yaml:"size"`
		UseTPM         bool   `yaml:"useTPM"`
	} `yaml:"settings"`
}

// LoadConfig reads the YAML configuration file and returns a BootstrapToken
