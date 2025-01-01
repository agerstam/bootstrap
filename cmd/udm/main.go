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

	volumePath := "udm-luks.img"
	password := "MyStr0ngP@assword!"
	size := 10
	mapperName := "udm-luks"
	mountpoint := "mnt/udm-luks"

	// Parse command line flags
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	authorizeFile := flag.String("authorize", "", "Authorize a node, requires path to the YAML configuration file")
	deauthorize := flag.Bool("deauthorize", false, "Deauthorize the node")

	flag.Parse()
	context := AppConfig{Verbose: *verbose}

	if *authorizeFile != "" && *deauthorize {
		log.Fatal("You must provide either the -authorize or -deauthorize flag, not both")
	} else if *authorizeFile != "" {
		handleAuthorize(*authorizeFile, context)

		fmt.Println("Bootstrap: Creating LUKS volume ...")
		if err := luks.CreateLUKSVolume(volumePath, password, size, false); err != nil {
			log.Fatalf("Failed to create LUKS volume: %v", err)
		}

		fmt.Println("Bootstrap: Opening LUKS volume ...")
		if err := luks.OpenLUKSVolume(volumePath, password, mapperName); err != nil {
			log.Fatalf("Failed to open LUKS volume: %v", err)
		}

		fmt.Println("Bootstrap: Formatting LUKS volume ...")
		if err := luks.FormatLUKSVolume(mapperName); err != nil {
			log.Fatalf("Failed to format LUKS volume: %v", err)
		}

		fmt.Println("Bootstrap: Mounting LUKS volume ...")
		if err := luks.MountLUKSVolume(mapperName, mountpoint); err != nil {
			log.Fatalf("Failed to mount LUKS volume: %v", err)
		}
		fmt.Println("Bootstrap: LUKS volume mounted successfully")

	} else if *deauthorize {
		handleDeauthorize(context)
	} else {
		// No valid flag
		log.Fatal("You must provide either the -authorize or -deauthorize flag")
	}
	if *verbose {
		log.Println("Verbose logging enabled")
	}

	// Setup signal handling for graceful shutdown
	setupSignalHandler(mapperName, mountpoint)

	fmt.Printf("LUKS volume successfully mounted at %s\n", mountpoint)
	fmt.Println("Press Ctrl+C to exit and clean up.")
	select {} // Wait indefinitely
}

func handleDeauthorize(context AppConfig) {
	fmt.Println("Bootstrap: Deauthorizing node")
}

func handleAuthorize(filePath string, context AppConfig) {
	fmt.Printf("Bootstrap: Authorizing node using file: %s\n", filePath)

	// Check if the file parameter was provided
	if filePath == "" {
		log.Fatal("You must provdie the path to the YML bootstrap file using the -file flat")
	}

	// Load configuration
	cfg, err := config.LoadConfig(filePath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	fmt.Printf("Bootstrap Token: \n")
	fmt.Printf("Token-Id: %s\n", cfg.Bootstrap.TokenId)
	fmt.Printf("Version: %s\n", cfg.Bootstrap.Version)
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
