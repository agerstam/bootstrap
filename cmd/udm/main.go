package main

import (
	"bootstrap/internal/config"
	"bootstrap/internal/luks"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
)

func printHelp() {
	fmt.Println("Usage: bootstrap [COMMAND] [OPTIONS]")
	fmt.Println("\nCommands:")
	fmt.Println("  --authorize --bootstrap=file --config=config.yml --keyfile=key.bin")
	fmt.Println("                                  Authorize with a required bootstrap file and output keyfile")
	fmt.Println("  --deauthorize --config=config.yml")
	fmt.Println("                                  Deauthorize with the specified config")
	fmt.Println("  --mount --config=config.yml --keyfile=key.bin")
	fmt.Println("                                  Mount a keyfile with the specified config")
	fmt.Println("  --unmount --config=config.yml")
	fmt.Println("                                  Unmount a configuration")
	fmt.Println("  --addPersistentMount --config=config.yml --keyfile=key.bin")
	fmt.Println("                                  Add a persistent mount with the specified config and keyfile")
	fmt.Println("  --removePersistentMount --config=config.yml")
	fmt.Println("                                  Remove a persistent mount with the specified config")
	fmt.Println("\nOptions:")
	fmt.Println("  --config=config.yml             Path to the configuration file (required for all commands)")
	fmt.Println("  --keyfile=key.bin               Path to the keyfile (output for --authorize, input for other commands)")
	fmt.Println("\nRun 'configapp --help' to display this help message.")
}
func main() {
	// Parse command line flags
	cmd := config.ParseCommandLine()

	// Read and parse the settings file
	cfg, err := config.LoadConfig(cmd.Config)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	printLUKSConfig(cfg)
	cfg.Cmd = cmd

	switch cfg.Cmd.CommandName {
	case "authorize":
		if !cfg.LUKS.UseTPM && len(cfg.Cmd.Keyfile) == 0 {
			log.Fatalf("Error: --keyfile must be specified when TPM is not used")
		}
		authorize(cfg)
	case "deauthorize":
		deauthorize(cfg)
	case "mount":
		mount(cfg)
	case "unmount":
		unmount(cfg)
	case "addPersistentMount":
		addPersistentMount(cfg)
	case "removePersistentMount":
		removePersistentMount(cfg)
	case "help":
		printHelp()
	default:
		printHelp()
		os.Exit(1)
	}
}

// Authorize and setup the LUKS volume
func authorize(cfg *config.AppConfig) {
	fmt.Println("Authorizing with config:", cfg.Cmd.Config)

	// Read and parse the bootstrap token file
	readBootstrapToken(cfg.Cmd.Bootstrap)

	// Setup LUKS volume
	if err := luks.SetupLUKSVolume(&cfg.LUKS); err != nil {
		log.Fatalf("Failed to setup LUKS volume: %v", err)
	}

	if !cfg.LUKS.UseTPM {
		writeKeyToFile(cfg.Cmd.Keyfile, cfg.LUKS.Password)
		fmt.Println("LUKS volume created, generated keyfile:", cfg.Cmd.Keyfile)
	} else {
		fmt.Println("LUKS volume created, using TPM for key storage NVIndex =", luks.DefaultNVIndex)
	}
}

func deauthorize(cfg *config.AppConfig) {
	fmt.Println("Deauthorizing with config:", cfg.Cmd.Config)

	// Remove LUKS volume
	if err := luks.RemoveLUKSVolume(&cfg.LUKS); err != nil {
		log.Printf("Error cleaning up LUKS volume: %v", err)
	}
	os.Exit(0)
}

func mount(cfg *config.AppConfig) {
	fmt.Println("Mounting with config:", cfg.Cmd.Config, "and keyfile:", cfg.Cmd.Keyfile)

	if !cfg.LUKS.UseTPM {
		// Read the keyfile
		key, err := readKeyFromFile(cfg.Cmd.Keyfile)
		if err != nil {
			log.Fatalf("Failed to read key from file: %v", err)
		}
		cfg.LUKS.Password = key
	}
	// Open LUKS Volume
	if err := luks.OpenLUKSVolume(&cfg.LUKS); err != nil {
		log.Fatalf("Failed to open LUKS volume: %v", err)
	}

	// Mount LUKS Volume
	if err := luks.MountLUKSVolume(&cfg.LUKS); err != nil {
		log.Fatalf("Failed to mount LUKS volume: %v", err)
	}

	fmt.Println("Mouned LUKS successfully:", cfg.LUKS.MountPoint)
}

func unmount(cfg *config.AppConfig) {
	fmt.Println("Unmounting with config:", cfg.Cmd.Config)

	// Unmount LUKS volume
	if err := luks.UnmountAndCloseLUKSVolume(&cfg.LUKS); err != nil {
		log.Fatalf("Error cleaning up LUKS volume: %v", err)
	}
}

func addPersistentMount(cfg *config.AppConfig) {
	fmt.Println("Adding persistent mount with config:", cfg.Cmd.Config, "and keyfile:", cfg.Cmd.Keyfile)

	// Add Persistent Mount
	if err := luks.AddPersistentMount(&cfg.LUKS, cfg.Cmd.Keyfile); err != nil {
		log.Fatalf("Failed to configure persistent mount: %v", err)
	}
}

func removePersistentMount(cfg *config.AppConfig) {
	fmt.Println("Removing persistent mount with config:", cfg.Cmd.Config)

	// Remove Persistent Mount
	if err := luks.RemovePersistentMount(&cfg.LUKS); err != nil {
		log.Fatalf("Failed to remove persistent mount: %v", err)
	}
}

func readBootstrapToken(filePath string) (token *config.BootstrapToken) {

	// Load bootstrap from file
	token, err := config.LoadBootstrap(filePath)
	if err != nil {
		log.Fatalf("Failed to load bootstrap file: %v", err)
	}

	// Validate
	if err := token.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	printBootstrapToken(token)
	return token
}

// writeKeyToFile writes the Key field from the LUKS structure to the specified binary file.
func writeKeyToFile(keyfile string, password []byte) error {

	// Validate that the Key field is not empty
	if len(password) == 0 {
		return fmt.Errorf("key field in LUKS structure is empty")
	}

	// Open the file for writing
	file, err := os.Create(keyfile)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write the Key field to the file
	_, err = file.Write(password)
	if err != nil {
		return fmt.Errorf("failed to write key to file: %w", err)
	}

	return nil
}

// readKeyFromFile reads the contents of a key file and validates it using a password.
func readKeyFromFile(keyfile string) ([]byte, error) {
	// Open the key file for reading
	file, err := os.Open(keyfile)
	if err != nil {
		return nil, fmt.Errorf("failed to open key file: %w", err)
	}
	defer file.Close()

	// Read the entire file content
	keyData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Validate key data (example: check length, match password, etc.)
	if len(keyData) == 0 {
		return nil, fmt.Errorf("key file is empty")
	}

	return keyData, nil
}

func printLUKSConfig(cfg *config.AppConfig) {

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Property", "Value"})
	t.AppendRows([]table.Row{
		{"Volume Path", cfg.LUKS.VolumePath},
		{"Mapper Name", cfg.LUKS.MapperName},
		{"Mount Point", cfg.LUKS.MountPoint},
		{"Password Length", cfg.LUKS.PasswordLength},
		{"Size", cfg.LUKS.Size},
		{"Use TPM", cfg.LUKS.UseTPM},
	})
	t.Render()
}

func printBootstrapToken(token *config.BootstrapToken) {

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Property", "Value"})
	t.AppendRows([]table.Row{
		{"Token ID", token.Bootstrap.TokenId},
		{"Version", token.Bootstrap.Version},
	})
	t.Render()
}
