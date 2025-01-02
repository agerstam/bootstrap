package luks

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/exec"
)

type LUKS struct {
	VolumePath     string `yaml:"volumePath"`
	MapperName     string `yaml:"mapperName"`
	MountPoint     string `yaml:"mountPoint"`
	PasswordLength int    `yaml:"passwordLength"`
	Size           int    `yaml:"size"`
	UseTPM         bool   `yaml:"useTPM"`
	Password       string `yaml:"-"`
}

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

	fmt.Println("Bootstrap: Creating LUKS volume ...")
	if err := CreateLUKSVolume(cfg.VolumePath, password, cfg.Size, cfg.UseTPM); err != nil {
		log.Fatalf("Failed to create LUKS volume: %v", err)
	}

	fmt.Println("Bootstrap: Opening LUKS volume ...")
	if err := OpenLUKSVolume(cfg.VolumePath, password, cfg.MapperName); err != nil {
		log.Fatalf("Failed to open LUKS volume: %v", err)
	}

	fmt.Println("Bootstrap: Formatting LUKS volume ...")
	if err := FormatLUKSVolume(cfg.MapperName); err != nil {
		log.Fatalf("Failed to format LUKS volume: %v", err)
	}

	fmt.Println("Bootstrap: Mounting LUKS volume ...")
	if err := mountLUKSVolume(cfg.MapperName, cfg.MountPoint); err != nil {
		log.Fatalf("Failed to mount LUKS volume: %v", err)
	}
	return nil
}

func UnmountAndCloseLUKSVolume(cfg *LUKS) error {
	if cfg == nil {
		return fmt.Errorf("LUKS configuration is nil")
	}

	fmt.Println("Unmounting LUKS volume...")
	if err := unmountLUKSVolume(cfg.MountPoint); err != nil {
		log.Printf("Failed to unmount LUKS volume: %v", err)
	}

	fmt.Println("Closing LUKS volume...")
	if err := closeLUKSVolume(cfg.MapperName); err != nil {
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

		if err := storePasswordInTPM(password, "0x1500016"); err != nil {
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
func OpenLUKSVolume(volumePath, password, mapperName string) error {

	mappedDevice := "/dev/mapper/" + mapperName

	// Check if the mapping already exists
	if _, err := os.Stat(mappedDevice); err == nil {
		// If the device exists, close it first
		cmd := exec.Command("cryptsetup", "luksClose", mapperName)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to close existing mapping: %s\n%s", err, string(output))
		}
	}

	cmd := exec.Command("cryptsetup", "luksOpen", volumePath, mapperName)
	cmd.Stdin = createPasswordInput(password, true)
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
	if err := unmountLUKSVolume(cfg.MountPoint); err != nil {
		return fmt.Errorf("failed to unmount LUKS volume: %w", err)
	}

	fmt.Println("Closing LUKS volume...")
	if err := closeLUKSVolume(cfg.MapperName); err != nil {
		return fmt.Errorf("failed to close LUKS volume: %w", err)
	}

	fmt.Println("Removing mount directory...")
	if err := os.RemoveAll(cfg.MountPoint); err != nil {
		return fmt.Errorf("failed to remove mount directory: %w", err)
	}

	fmt.Println("Removing LUKS image file ...")
	if err := os.Remove(cfg.VolumePath); err != nil {
		return fmt.Errorf("failed to remove LUKS image file: %w", err)
	}
	if cfg.UseTPM {
		fmt.Println("Removing password from TPM ...")
		if err := removePasswordFromTPM(DefaultNVIndex); err != nil {
			return fmt.Errorf("failed to remove password from TPM: %w", err)
		}
	}
	return nil
}

// MountLUKSVolume mounts the mapped LUKS volume to the specified mount point
func mountLUKSVolume(mapperName, mountPoint string) error {
	devicePath := "/dev/mapper/" + mapperName
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	cmd := exec.Command("mount", devicePath, mountPoint)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to mount LUKS volume: %s", output)
	}
	return nil
}

// unmountLUKSVolume unmounts the mapped LUKS volume
func unmountLUKSVolume(mountPoint string) error {
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
func closeLUKSVolume(mapperName string) error {
	cmd := exec.Command("cryptsetup", "luksClose", mapperName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to close LUKS volume: %s", output)
	}
	return nil
}

// createSparseFile creates a sparse file of the specified size
func createSparseFile(filePath string, sizeMB int) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	sizeBytes := int64(sizeMB) * 1024 * 1024
	if err := file.Truncate((sizeBytes)); err != nil {
		return fmt.Errorf("failed to truncate file: %w", err)
	}
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

// retrievePasswordFromTPM retrieves the LUKS password from the TPM.
func retrievePasswordFromTPM() (string, error) {
	cmd := exec.Command("tpm2_nvread", "--index=0x1500016", "--size=64")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tpm2_nvread error: %w", err)
	}
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
