package main

import (
	"bootstrap/internal/config"
	"bootstrap/internal/luks"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	// Read and parse the settings file
	cfg := readConfig("config.yml")

	// Parse command line flags
	cfg.Verbose = flag.Bool("verbose", false, "Enable verbose logging")
	cfg.BootstrapFile = flag.String("authorize", "", "Authorize a node, requires path to the YAML configuration file")
	cfg.Deauthorize = flag.Bool("deauthorize", false, "Deauthorize the node")
	flag.Parse()

	if *cfg.BootstrapFile != "" && *cfg.Deauthorize {
		log.Fatal("You must provide either the -authorize or -deauthorize flag, not both")
	} else if *cfg.BootstrapFile != "" {

		// Read and parse the bootstrap token file
		readBootstrapToken(*cfg.BootstrapFile)

		// Setup LUKS volume
		if err := luks.SetupLUKSVolume(&cfg.LUKS); err != nil {
			log.Fatalf("Failed to setup LUKS volume: %v", err)
		}
		fmt.Println("Bootstrap: LUKS volume mounted successfully")

	} else if *cfg.Deauthorize {
		deauthorizeNode(cfg)
	} else {
		// No valid flag
		log.Fatal("You must provide either the -authorize or -deauthorize flag")
	}
	if *cfg.Verbose {
		log.Println("Bootstrap: Verbose logging enabled")
	}

	// Setup signal handling for graceful shutdown
	setupSignalHandler(cfg)

	fmt.Printf("LUKS volume successfully mounted at %s\n", cfg.LUKS.MountPoint)
	fmt.Println("Press Ctrl+C to exit and clean up.")
	select {} // Wait FOREVER
}

func deauthorizeNode(cfg *config.AppConfig) {
	fmt.Println("Bootstrap: Deauthorizing node")

	if err := luks.RemoveLUKSVolume(&cfg.LUKS); err != nil {
		log.Printf("Error cleaning up LUKS volume: %v", err)
	}
	os.Exit(0)
}

func readBootstrapToken(filePath string) (token *config.BootstrapToken) {
	fmt.Printf("Bootstrap: Authorizing node using file: %s\n", filePath)

	// Check if the file parameter was provided
	if filePath == "" {
		log.Fatal("You must provdie the path to the YML bootstrap file using the -file flat")
	}

	// Load configuration
	token, err := config.LoadBootstrap(filePath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate
	if err := token.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	fmt.Printf("Bootstrap Token: \n")
	fmt.Printf("   Token ID: %s\n", token.Bootstrap.TokenId)
	fmt.Printf("   Version: %s\n", token.Bootstrap.Version)

	return token
}

func readConfig(filePath string) *config.AppConfig {
	fmt.Printf("Bootstrap: Reading settings from file: %s\n", filePath)

	// Load configuration
	cfg, err := config.LoadSettings(filePath)
	if err != nil {
		log.Printf("Failed to load configuration (using defaults): %v", err)
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	fmt.Printf("LUKS Config: \n")
	fmt.Printf("   Volume Path: %s \n", cfg.LUKS.VolumePath)
	fmt.Printf("   Mapper Name: %s \n", cfg.LUKS.MapperName)
	fmt.Printf("   Mount Point: %s \n", cfg.LUKS.MountPoint)
	fmt.Printf("   Password Length: %d \n", cfg.LUKS.PasswordLength)
	fmt.Printf("   Size: %d \n", cfg.LUKS.Size)
	fmt.Printf("   Use TPM: %t \n", cfg.LUKS.UseTPM)

	return cfg
}

// setupSignalHandler ensures cleanup happens on program exit.
func setupSignalHandler(cfg *config.AppConfig) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		fmt.Println("\nReceived interrupt. Cleaning up...")

		if err := luks.UnmountAndCloseLUKSVolume(&cfg.LUKS); err != nil {
			log.Printf("Error cleaning up LUKS volume: %v", err)
		}
		os.Exit(0)
	}()
}
