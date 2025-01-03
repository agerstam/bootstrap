package config

import (
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func ParseCommandLine() Command {
	var cmd Command

	// Define flags
	authorize := flag.Bool("authorize", false, "Authorize with a bootstrap file and configuration")
	bootstrap := flag.String("bootstrap", "", "Path to bootstrap YAML (required for --authorize)")
	config := flag.String("config", "", "Path to config YAML")
	deauthorize := flag.Bool("deauthorize", false, "Deauthorize")
	mount := flag.Bool("mount", false, "Mount a keyfile")
	unmount := flag.Bool("unmount", false, "Unmount a configuration")
	addPersistentMount := flag.Bool("addPersistentMount", false, "Add a persistent mount")
	removePersistentMount := flag.Bool("removePersistentMount", false, "Remove a persistent mount")
	keyfile := flag.String("keyfile", "", "Path to keyfile")

	// Parse flags
	flag.Parse()

	// Validate that --config is provided for all cases
	if *config == "" {
		fmt.Println("Error: --config is required for all commands")
		os.Exit(1)
	}

	// Determine command based
	switch {
	case *authorize:
		if *bootstrap == "" || *keyfile == "" {
			fmt.Println("Error: --bootstrap and --keyfile are required for --authorize")
			os.Exit(1)
		}
		cmd.CommandName = "authorize"
		cmd.Bootstrap = *bootstrap
	case *deauthorize:
		cmd.CommandName = "deauthorize"
	case *mount:
		cmd.CommandName = "mount"
	case *unmount:
		cmd.CommandName = "unmount"
	case *addPersistentMount:
		cmd.CommandName = "addPersistentMount"
	case *removePersistentMount:
		cmd.CommandName = "removePersistentMount"
	default:
		cmd.CommandName = "help"
	}

	// Assign common flag values to the command structure
	cmd.Config = *config
	cmd.Keyfile = *keyfile

	return cmd
}

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

func LoadConfig(filePath string) (*AppConfig, error) {
	fmt.Printf("Bootstrap: Reading settings from file: %s\n", filePath)

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

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %v", err)
	}
	return &cfg, nil
}

func (cfg *AppConfig) Validate() error {

	if cfg.LUKS.VolumePath == "" {
		return fmt.Errorf("luks.volume-path is required")
	}
	if cfg.LUKS.MapperName == "" {
		return fmt.Errorf("luks.mapper-name is required")
	}
	if cfg.LUKS.MountPoint == "" {
		return fmt.Errorf("luks.mount-point is required")
	}
	if cfg.LUKS.PasswordLength == 0 {
		return fmt.Errorf("luks.password-length is required")
	}
	if cfg.LUKS.Size == 0 {
		return fmt.Errorf("luks.size (MB) is required")
	}
	if cfg.LUKS.User == "" {
		cfg.LUKS.User = "root" // default value
	}
	if cfg.LUKS.Group == "" {
		cfg.LUKS.Group = "root" // default value
	}
	return nil
}
