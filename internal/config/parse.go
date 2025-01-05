package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

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

	// If no --config is provided, try loading config.yml from the current directory
	if *config == "" {
		defaultConfigPath := filepath.Join(getCurrentDirectory(), "config.yml")
		if _, err := os.Stat(defaultConfigPath); os.IsNotExist(err) {
			fmt.Println("Error: --config is required and no default config.yml found in the current directory")
			os.Exit(1)
		}
		*config = defaultConfigPath
	}

	// Determine command based
	switch {
	case *authorize:
		if *keyfile == "" {
			fmt.Println("Error: --keyfile is required for --authorize")
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

// LoadBootstrap attempts to load the BootstrapToken from an environment variable,
// then falls back to a file if the environment variable is not set.
func LoadBootstrap(filePath string) (*BootstrapToken, error) {
	var token BootstrapToken

	// Attempt to load from the environment variable
	envData := os.Getenv("BOOTSTRAP_YML")
	fmt.Println("envData:", envData)
	if envData != "" {
		if err := yaml.Unmarshal([]byte(envData), &token); err != nil {
			return nil, fmt.Errorf("failed to parse YAML from environment variable: %w", err)
		}
		return &token, nil
	}

	// Fallback to loading from the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Parse the YAML file
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
	fmt.Printf("Reading settings from file: %s\n", filePath)

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

// Helper function to get the current directory of the executable
func getCurrentDirectory() string {
	execPath, err := os.Executable()
	if err != nil {
		fmt.Printf("Error determining executable path: %v\n", err)
		os.Exit(1)
	}
	return filepath.Dir(execPath)
}
