package luks

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type LUKS struct {
	VolumePath     string `yaml:"volumePath"`
	MapperName     string `yaml:"mapperName"`
	MountPoint     string `yaml:"mountPoint"`
	PasswordLength int    `yaml:"passwordLength"`
	Size           int    `yaml:"size"`
	UseTPM         bool   `yaml:"useTPM"`
	User           string `yaml:"user"`
	Group          string `yaml:"group"`
	Password       []byte `yaml:"-"`
} // `yaml:"luks"`

const DefaultNVIndex = "0x1500016"

// checkTPM2Availability determines if TPM 2.0 is available on the system.
func checkTPM2Availability() (bool, error) {
	const tpm2Device = "/dev/tpmrm0" // Device file for TPM 2.0

	if _, err := os.Stat(tpm2Device); err == nil {
		return true, nil // TPM 2.0 is available
	} else if errors.Is(err, os.ErrNotExist) {
		return false, nil // TPM 2.0 is not available
	} else {
		return false, fmt.Errorf("error accessing TPM 2.0 device: %w", err)
	}
}

// SetupLUKSVolume sets up and mounts a new LUKS volume
func SetupLUKSVolume(cfg *LUKS) error {

	if cfg == nil {
		return fmt.Errorf("LUKS configuration is nil")
	}

	if cfg.UseTPM {
		isTPM2Available, err := checkTPM2Availability()
		if err != nil {
			log.Printf("error checking TPM 2.0 availability: %v\n", err)
		} else if !isTPM2Available {
			return fmt.Errorf("TPM 2.0 not availabile on this system, reconfigure to use keyfile")
		}
	}

	// Generate high entropy password
	password, err := GenerateLUKSKey(cfg.PasswordLength)
	if err != nil {
		log.Fatalf("Failed to generate password: %v", err)
	}
	cfg.Password = password

	fmt.Println("Creating LUKS volume ...")
	if err := CreateLUKSVolume(cfg.VolumePath, password, cfg.Size, cfg.UseTPM); err != nil {
		log.Fatalf("Failed to create LUKS volume: %v", err)
	}

	fmt.Println("Opening LUKS volume ...")
	if err := OpenLUKSVolume(cfg); err != nil {
		log.Fatalf("Failed to open LUKS volume: %v", err)
	}

	fmt.Println("Formatting LUKS volume ...")
	if err := FormatLUKSVolume(cfg.MapperName); err != nil {
		log.Fatalf("Failed to format LUKS volume: %v", err)
	}

	fmt.Println("Mounting LUKS volume ...")
	if err := MountLUKSVolume(cfg); err != nil {
		log.Fatalf("Failed to mount LUKS volume: %v", err)
	}

	return nil
}

func UnmountAndCloseLUKSVolume(cfg *LUKS) error {
	if cfg == nil {
		return fmt.Errorf("LUKS configuration is nil")
	}

	fmt.Println("Unmounting LUKS volume...")
	if err := UnmountLUKSVolume(cfg.MountPoint); err != nil {
		log.Printf("Failed to unmount LUKS volume: %v", err)
	}

	fmt.Println("Closing LUKS volume...")
	if err := CloseLUKSVolume(cfg.MapperName); err != nil {
		log.Printf("Failed to close LUKS volume: %v", err)
	}

	return nil
}

// CreateLUKSVolume set up a new LUKS volume with the specified size and password
func CreateLUKSVolume(filePath string, password []byte, sizeMB int, useTPM bool) error {

	if sizeMB < 1 || sizeMB > 64 {
		return fmt.Errorf("size must be between 1MB and 10MB")
	}

	// Create a sparse file of the specified size
	if err := createSparseFile(filePath, sizeMB); err != nil {
		return fmt.Errorf("failed to create sparse file: %w", err)
	}

	// Optionally store the password in the TPM
	if useTPM {

		// Remove the password from the TPM if it already exists
		if err := removePasswordFromTPM(DefaultNVIndex); err != nil {
			log.Printf("failed to remove existing password from TPM: %s", err)
		}

		if err := storePasswordInTPM(password, DefaultNVIndex); err != nil {
			return fmt.Errorf("failed to store password in TPM: %w", err)
		}
	}

	// Format the file as a LUKS volume
	if err := luksFormat(filePath, password); err != nil {
		return fmt.Errorf("failed to format LUKS volume: %w", err)
	}

	return nil
}

// OpenLUKSVolume opens an existing LUKS volume
func OpenLUKSVolume(cfg *LUKS) error {

	mappedDevice := "/dev/mapper/" + cfg.MapperName

	// Check if the mapping already exists
	if _, err := os.Stat(mappedDevice); err == nil {
		// If the device exists, close it first
		cmd := exec.Command("cryptsetup", "luksClose", cfg.MapperName)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to close existing mapping: %s\n%s", err, string(output))
		}
	}

	if cfg.UseTPM {

		// Retrieve the password from the TPM
		password, err := retrievePasswordFromTPM(DefaultNVIndex, cfg.PasswordLength)
		if err != nil {
			return fmt.Errorf("failed to retrieve password from TPM: %w", err)
		}
		cfg.Password = password
	}

	cmd := exec.Command("cryptsetup", "luksOpen", cfg.VolumePath, cfg.MapperName)
	cmd.Stdin = createPasswordInput(cfg.Password, true)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to open LUKS volume: %s", output)
	}
	return nil
}

// FormatLuksVolume formats an existing LUKS volume
func FormatLUKSVolume(mapperName string) error {
	devicePath := "/dev/mapper/" + mapperName
	cmd := exec.Command("mkfs.ext4", devicePath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to format LUKS volume: %s", output)
	}

	return nil
}

// CleanupLUKSVolume unmounts and closes the LUKS volume and removes the mount point
func RemoveLUKSVolume(cfg *LUKS) error {
	fmt.Println("Unmounting LUKS volume...")
	if err := UnmountLUKSVolume(cfg.MountPoint); err != nil {
		log.Printf("failed to unmount LUKS volume: %s", err)
	}

	fmt.Println("Closing LUKS volume...")
	if err := CloseLUKSVolume(cfg.MapperName); err != nil {
		log.Printf("failed to close LUKS volume: %s", err)
	}

	fmt.Println("Removing mount directory...")
	if err := os.RemoveAll(cfg.MountPoint); err != nil {
		log.Printf("failed to remove mount directory: %s", err)
	}

	fmt.Println("Removing LUKS image file ...")
	if err := os.Remove(cfg.VolumePath); err != nil {
		log.Printf("failed to remove LUKS image file: %s", err)
	}
	if cfg.UseTPM {
		fmt.Println("Removing password from TPM ...")
		if err := removePasswordFromTPM(DefaultNVIndex); err != nil {
			log.Printf("failed to remove password from TPM: %s", err)
		}
	}
	return nil
}

// MountLUKSVolume mounts the mapped LUKS volume to the specified mount point
func MountLUKSVolume(cfg *LUKS) error { //mapperName, mountPoint, user, group string) error {
	devicePath := "/dev/mapper/" + cfg.MapperName
	if err := os.MkdirAll(cfg.MountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	cmd := exec.Command("mount", devicePath, cfg.MountPoint)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to mount LUKS volume: %s", output)
	}

	// Change ownership of the mount point
	if cfg.User == "" || cfg.Group == "" {
		return fmt.Errorf(("user and group must be specified"))
	}
	cmd = exec.Command("chown", fmt.Sprintf("%s:%s", cfg.User, cfg.Group), cfg.MountPoint)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to change ownership of mount point: %s\n%s", err, string(output))
	}

	return nil
}

// unmountLUKSVolume unmounts the mapped LUKS volume
func UnmountLUKSVolume(mountPoint string) error {
	cmd := exec.Command("umount", mountPoint)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Retry with lazy unmount
		fmt.Printf("Normal unmount failed: %s. Retrying with lazy unmount...\n", err)
		cmd = exec.Command("umount", "-l", mountPoint)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to unmount LUKS volume: %s\n%s", err, string(output))
		}
	}
	return nil
}

// CloseLUKSVolume closes the mapped LUKS volume
func CloseLUKSVolume(mapperName string) error {
	cmd := exec.Command("cryptsetup", "luksClose", mapperName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to close LUKS volume: %s", output)
	}
	return nil
}

// createSparseFile creates a sparse file of the specified size in MB
func createSparseFile(filePath string, sizeMB int) error {
	// Extract the directory path
	dir := filepath.Dir(filePath)

	// Ensure the directory exists
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Create the file with secure permissions
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer file.Close()

	// Calculate the size in bytes
	sizeBytes := int64(sizeMB) * 1024 * 1024

	// Truncate the file to the desired size (creates a sparse file)
	if err := file.Truncate(sizeBytes); err != nil {
		return fmt.Errorf("failed to truncate file %s: %w", filePath, err)
	}

	return nil
}

// luksFormat formats the file as a LUKS volume
func luksFormat(filePath string, password []byte) error {
	// Create a temporary file to store the password
	tmpFile, err := os.CreateTemp("", "luks-password-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name()) // Ensure the file is removed after use

	// Write the password to the temporary file
	if _, err := tmpFile.Write(password); err != nil {
		return fmt.Errorf("failed to write password to temporary file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	cmd := exec.Command(
		"cryptsetup",
		"luksFormat",
		"--type=luks2",
		"--batch-mode",
		"--pbkdf-memory=2097152",
		"--pbkdf-parallel=8",
		"--cipher=aes-xts-plain64",
		"--key-file", tmpFile.Name(),
		filePath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to format LUKS volume: %s, error: %w", output, err)
	}
	return nil
}

// createPasswordInput creates a pipe to provide the password as input.
func createPasswordInput(password []byte, addNewline bool) *os.File {
	r, w, _ := os.Pipe()

	go func() {
		defer w.Close()
		if addNewline {
			w.Write(append(password, '\n'))
		} else {
			w.Write(password)
		}
	}()

	return r
}

// storePasswordInTPM stores the LUKS password securely in the TPM.
func storePasswordInTPM(password []byte, nvIndex string) error {

	// Validate password length
	//passwordLength := len(password)
	if len(password) < 1 || len(password) > 64 {
		return fmt.Errorf("password length (%d bytes) must be between 1 and 64 bytes", len(password))
	}

	// Define the NV index with the password length as the size
	cmd := exec.Command("tpm2_nvdefine",
		nvIndex,
		fmt.Sprintf("--size=%d", len(password)),
		"--attributes=ownerread|ownerwrite|authread|authwrite")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tpm2_nvdefine error: %s", string(output))
	}

	// Write the password to the NV index
	cmd = exec.Command("tpm2_nvwrite",
		nvIndex,
		"--input=-") // Use stdin for the input
	cmd.Stdin = createPasswordInput(password, false)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tpm2_nvwrite error: %s", string(output))
	}

	return nil
}

// removePasswordFromTPM removes the LUKS password from the specified NV index in the TPM.
func removePasswordFromTPM(nvIndex string) error {
	cmd := exec.Command("tpm2_nvundefine", nvIndex)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tpm2_nvundefine error: %s", string(output))
	}
	return nil
}

// retrievePasswordFromTPM retrieves the LUKS password from the TPM for the specified NV index and size.
func retrievePasswordFromTPM(nvindex string, size int) ([]byte, error) {

	// Construct the tpm2_nvread command with the provided NV index and size
	cmd := exec.Command("tpm2_nvread", nvindex, fmt.Sprintf("--size=%d", size))

	// Execute the command and capture the output
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tpm2_nvread error for index %s: %w", nvindex, err)
	}

	// Return the output as a string
	return output, nil
}

// GenerateLUKSKey generates a random key of the specified length in bytes,
// using tpm2_getrandom if available, otherwise falling back to crypto/rand.
func GenerateLUKSKey(length int) ([]byte, error) {

	if length <= 8 {
		return nil, fmt.Errorf("key length must be greater than 0")
	}

	// Check if tpm2_getrandom is available.
	isTPMAvailable, err := checkTPM2Availability()
	if err != nil {
		log.Printf("Error when checking TPM device")
	} else if isTPMAvailable {
		key, err := getRandomBytesFromTPM2(length)
		if err == nil {
			return key, nil
		}
		fmt.Printf("Failed to use TPM: %v. Falling back to crypto/rand.\n", err)
	}
	// Fallback to crypto/rand.
	key := make([]byte, length)
	_, err = rand.Read(key)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random key using crypto/rand: %w", err)
	}
	return key, nil
}

// getRandomBytesFromTPM2 fetches the specified number of random bytes using tpm2_getrandom.
func getRandomBytesFromTPM2(size int) ([]byte, error) {

	// Execute the tpm2_getrandom command to fetch `size` bytes in hex format.
	cmd := exec.Command("tpm2_getrandom", fmt.Sprintf("%d", size), "--hex")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to execute tpm2_getrandom: %w", err)
	}

	// Parse the output as a hex string.
	trimmedOutput := strings.TrimSpace(out.String())
	randomBytes, err := hex.DecodeString(trimmedOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to decode tpm2_getrandom output: %w", err)
	}

	return randomBytes, nil
}

func isLUKSMounted(cfg *LUKS) (bool, error) {
	devicePath := "/dev/mapper/" + cfg.MapperName

	cmd := exec.Command("lsblk", "-o", "MOUNTPOINT", "--noheadings", devicePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to list mounted devices: %s, error: %v", output, err)
	}
	return strings.TrimSpace(string(output)) == cfg.MountPoint, nil
}

// AddPersistentMount sets up the necessary entries in /etc/fstab for persistent mount
func AddPersistentMount(cfg *LUKS, keyFile string) error {

	isMounted, err := isLUKSMounted(cfg)
	if err != nil {
		return fmt.Errorf("failed to check if LUKS volume is mounted: %v", err)
	}
	if !isMounted {
		return fmt.Errorf("LUKS volume is not mounted")
	}

	// Update /etc/crypttab
	var crypttabEntry string
	if cfg.UseTPM {
		crypttabEntry = fmt.Sprintf("%s %s none luks,keyscript=/usr/local/bin/tpm-luks-keyscript.sh\n",
			cfg.MapperName, cfg.VolumePath)
	} else {
		crypttabEntry = fmt.Sprintf("%s %s %s luks\n", cfg.MapperName, cfg.VolumePath, keyFile)
	}

	if err := appendToFile("/etc/crypttab", crypttabEntry); err != nil {
		return fmt.Errorf("failed to update /etc/crypttab: %v", err)
	}

	devicePath := "/dev/mapper/" + cfg.MapperName
	filesystemUUID, err := getFilesystemUUID(devicePath)
	fmt.Printf("Filesystem UUID, mappedDevice (%s): %s\n", devicePath, filesystemUUID)
	if err != nil {
		return fmt.Errorf("failed to retrieve filesystem UUID: %w", err)
	}

	// Update /etc/fstab
	fstabEntry := fmt.Sprintf("UUID=%s %s ext4 defaults,nofail,x-systemd.requires=cryptsetup@%s.service 0 2\n",
		filesystemUUID, cfg.MountPoint, cfg.MapperName)

	if err := appendToFile("/etc/fstab", fstabEntry); err != nil {
		return fmt.Errorf("failed to update /etc/fstab: %v", err)
	}

	return nil
}

// RemovePersistentMount removes the entries in /etc/fstab for persistent mount
func RemovePersistentMount(cfg *LUKS) error {

	isMounted, err := isLUKSMounted(cfg)
	if err != nil {
		return fmt.Errorf("failed to check if LUKS volume is mounted: %v", err)
	}
	if isMounted {
		return fmt.Errorf("LUKS volume is mounted, please unmount first")
	}

	// Remove the entry from /etc/fstab
	if err := removeLineFromFile("/etc/fstab", cfg.MountPoint); err != nil {
		return fmt.Errorf("failed to remove entry from /etc/fstab: %v", err)
	}

	// Remove the entry from /etc/crypttab
	if err := removeLineFromFile("/etc/crypttab", cfg.MapperName); err != nil {
		return fmt.Errorf("failed to remove entry from /etc/crypttab: %v", err)
	}

	return nil
}

func getFilesystemUUID(devicePath string) (string, error) {

	// NOTE: the 'probe' option ensures we are getting the correct UUID
	log.Printf("Getting filesystem UUID for device: %s\n", devicePath)
	cmd := exec.Command("blkid", "-p", "-s", "UUID", "-o", "value", devicePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("blkid command failed: %s, output: %s", err, string(output))
	}
	uuid := strings.TrimSpace(string(output))
	if uuid == "" {
		return "", fmt.Errorf("no UUID found for device: %s", devicePath)
	}
	return uuid, nil
}

func removeLineFromFile(filePath, token string) error {
	// Open the original file for reading
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", filePath, err)
	}
	defer file.Close()

	// Create a temporary file in the same directory as the original file
	tempFilePath := filePath + ".tmp"
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempFilePath) // Clean up the temp file in case of an error
	}()

	scanner := bufio.NewScanner(file)
	writer := bufio.NewWriter(tempFile)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, token) {
			if _, err := writer.WriteString(line + "\n"); err != nil {
				return fmt.Errorf("failed to write to temporary file: %v", err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file %s: %v", filePath, err)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush data to temporary file: %v", err)
	}

	// Close the files before renaming
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %v", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close original file: %v", err)
	}

	// Replace the original file with the temporary file
	if err := os.Rename(tempFilePath, filePath); err != nil {
		return fmt.Errorf("failed to replace original file with temporary file: %v", err)
	}

	return nil
}

// appendToFile appends the given content to a file.
func appendToFile(filePath, content string) error {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return err
	}

	return nil
}
