package luks

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
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
	Password       string `yaml:"-"`
	nvIndex        string `yaml:"-"`
} // `yaml:"luks"`

const DefaultNVIndex = "0x1500016"

// SetupLUKSVolume sets up and mounts a new LUKS volume
func SetupLUKSVolume(cfg *LUKS) error {

	if cfg == nil {
		return fmt.Errorf("LUKS configuration is nil")
	}
	// Generate high entropy password
	password, err := GeneratePassword(cfg.PasswordLength)
	if err != nil {
		log.Fatalf("Failed to generate password: %v", err)
	}
	cfg.Password = password

	fmt.Println("Bootstrap: Creating LUKS volume ...")
	if err := CreateLUKSVolume(cfg.VolumePath, password, cfg.Size, cfg.UseTPM); err != nil {
		log.Fatalf("Failed to create LUKS volume: %v", err)
	}

	fmt.Println("Bootstrap: Opening LUKS volume ...")
	if err := OpenLUKSVolume(cfg); err != nil {
		log.Fatalf("Failed to open LUKS volume: %v", err)
	}

	fmt.Println("Bootstrap: Formatting LUKS volume ...")
	if err := FormatLUKSVolume(cfg.MapperName); err != nil {
		log.Fatalf("Failed to format LUKS volume: %v", err)
	}

	fmt.Println("Bootstrap: Mounting LUKS volume ...")
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
func CreateLUKSVolume(filePath string, password string, sizeMB int, useTPM bool) error {

	if sizeMB < 1 || sizeMB > 10 {
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
	fmt.Printf("Creating directory: %s\n", dir)

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
	fmt.Printf("Sparse file truncated to %d MB: %s\n", sizeMB, filePath)

	return nil
}

// luksFormat formats the file as a LUKS volume
func luksFormat(filePath, password string) error {
	cmd := exec.Command(
		"cryptsetup",
		"luksFormat",
		"--type=luks1",
		filePath,
	)
	cmd.Stdin = createPasswordInput(password, true)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to format LUKS volume: %s", output)
	}
	return nil
}

// createPasswordInput creates a pipe to provide the password as input.
func createPasswordInput(password string, addNewline bool) *os.File {
	r, w, _ := os.Pipe()

	go func() {
		defer w.Close()
		if addNewline {
			w.WriteString(password + "\n")
		} else {
			w.WriteString(password)
		}
	}()

	return r
}

// storePasswordInTPM stores the LUKS password securely in the TPM.
func storePasswordInTPM(password string, nvIndex string) error {

	// Validate password length
	passwordLength := len(password)
	if passwordLength < 1 || passwordLength > 64 {
		return fmt.Errorf("password length (%d bytes) must be between 1 and 64 bytes", passwordLength)
	}

	// Define the NV index with the password length as the size
	cmd := exec.Command("tpm2_nvdefine",
		nvIndex,
		fmt.Sprintf("--size=%d", passwordLength),
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
func retrievePasswordFromTPM(nvindex string, size int) (string, error) {

	// Construct the tpm2_nvread command with the provided NV index and size
	cmd := exec.Command("tpm2_nvread", nvindex, fmt.Sprintf("--size=%d", size))

	// Execute the command and capture the output
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tpm2_nvread error for index %s: %w", nvindex, err)
	}

	// Return the output as a string
	return string(output), nil
}

func GeneratePassword(length int) (string, error) {
	if length <= 0 || length > 64 {
		return "", fmt.Errorf("password length must be between 1 and 64")
	}

	// Define the character set for the password.
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+[]{}|;:,.<>?/"
	charsetLength := big.NewInt(int64(len(charset)))

	// Generate the password.
	password := make([]byte, length)
	for i := 0; i < length; i++ {
		charIndex, err := rand.Int(rand.Reader, charsetLength)
		if err != nil {
			return "", fmt.Errorf("failed to generate random character: %w", err)
		}
		password[i] = charset[charIndex.Int64()]
	}

	return string(password), nil
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

// ConfigurePersistentMount sets up the necessary entries in /etc/fstab for persistent mount
func ConfigurePersistentMount(cfg *LUKS, keyFile string) error {

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
		crypttabEntry = fmt.Sprintf("%s %s none luks,keyscript=/usr/local/bin/tpm-luks-keyscript %s\n", cfg.MapperName, cfg.VolumePath, DefaultNVIndex)
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

func getFilesystemUUID(devicePath string) (string, error) {

	log.Printf("Getting filesystem UUID for device: %s\n", devicePath)
	cmd := exec.Command("blkid", "-s", "UUID", "-o", "value", devicePath)
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

// removeLineFromFile removes lines containing the specified token from the given file.
func removeLineFromFile(filePath, token string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, token) {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	file, err = os.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		return err
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
