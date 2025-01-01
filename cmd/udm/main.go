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

type AppConfig struct {
	Verbose bool
}

func main() {

	// Parse command line flags
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	authorizeFile := flag.String("authorize", "", "Authorize a node, requires path to the YAML configuration file")
	deauthorize := flag.Bool("deauthorize", false, "Deauthorize the node")

	flag.Parse()
	context := AppConfig{Verbose: *verbose}

	// Read and parse the settings file
	s := readSettings("settings.yml", context)

	if *authorizeFile != "" && *deauthorize {
		log.Fatal("You must provide either the -authorize or -deauthorize flag, not both")
	} else if *authorizeFile != "" {

		// Read and parse the bootstrap token file
		readBootstrapToken(*authorizeFile, context)

		// Generate high entropy password
		password, err := GeneratePassword(s.Settings.PasswordLength)
		if err != nil {
			log.Fatalf("Failed to generate password: %v", err)
		}

		fmt.Println("Bootstrap: Creating LUKS volume ...")
		if err := luks.CreateLUKSVolume(s.Settings.VolumePath, password, s.Settings.Size, s.Settings.UseTPM); err != nil {
			log.Fatalf("Failed to create LUKS volume: %v", err)
		}

		fmt.Println("Bootstrap: Opening LUKS volume ...")
		if err := luks.OpenLUKSVolume(s.Settings.VolumePath, password, s.Settings.MapperName); err != nil {
			log.Fatalf("Failed to open LUKS volume: %v", err)
		}

		fmt.Println("Bootstrap: Formatting LUKS volume ...")
		if err := luks.FormatLUKSVolume(s.Settings.MapperName); err != nil {
			log.Fatalf("Failed to format LUKS volume: %v", err)
		}

		fmt.Println("Bootstrap: Mounting LUKS volume ...")
		if err := luks.MountLUKSVolume(s.Settings.MapperName, s.Settings.MountPoint); err != nil {
			log.Fatalf("Failed to mount LUKS volume: %v", err)
		}
		fmt.Println("Bootstrap: LUKS volume mounted successfully")

	} else if *deauthorize {
		deauthorizeNode(s.Settings.MapperName, s.Settings.MountPoint, context)
	} else {
		// No valid flag
		log.Fatal("You must provide either the -authorize or -deauthorize flag")
	}
	if *verbose {
		log.Println("Verbose logging enabled")
	}

	// Setup signal handling for graceful shutdown
	setupSignalHandler(s.Settings.MapperName, s.Settings.MountPoint)

	fmt.Printf("LUKS volume successfully mounted at %s\n", s.Settings.MountPoint)
	fmt.Println("Press Ctrl+C to exit and clean up.")
	select {} // Wait indefinitely
}

func deauthorizeNode(mapperName string, mountPoint string, context AppConfig) {
	fmt.Println("Bootstrap: Deauthorizing node")

	if err := luks.CleanupLUKSVolume(mapperName, mountPoint); err != nil {
		log.Printf("Error cleaning up LUKS volume: %v", err)
	}
	os.Exit(0)
}

func readBootstrapToken(filePath string, context AppConfig) (token *config.BootstrapToken) {
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
	fmt.Printf("Token-Id: %s\n", token.Bootstrap.TokenId)
	fmt.Printf("Version: %s\n", token.Bootstrap.Version)

	return token
}

func readSettings(filePath string, context AppConfig) (settings *config.AppSettings) {
	fmt.Printf("Bootstrap: Reading settings from file: %s\n", filePath)

	// Load configuration
	settings, err := config.LoadSettings(filePath)
	if err != nil {
		log.Printf("Failed to load configuration (using defaults): %v", err)
	}

	// Validate
	if err := settings.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	fmt.Printf("Settings: \n")
	fmt.Printf("Volume Path: %s\n", settings.Settings.VolumePath)
	fmt.Printf("Mapper Name: %s\n", settings.Settings.MapperName)
	fmt.Printf("Mount Point: %s\n", settings.Settings.MountPoint)
	fmt.Printf("Password Length: %d\n", settings.Settings.PasswordLength)
	fmt.Printf("Size: %s\n", settings.Settings.Size)
	fmt.Printf("Use TPM: %t\n", settings.Settings.UseTPM)

	return settings
}

// setupSignalHandler ensures cleanup happens on program exit.
func setupSignalHandler(mapperName, mountPoint string) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		fmt.Println("\nReceived interrupt. Cleaning up...")

		if err := luks.CleanupLUKSVolume(mapperName, mountPoint); err != nil {
			log.Printf("Error cleaning up LUKS volume: %v", err)
		}
		os.Exit(0)
	}()
}
