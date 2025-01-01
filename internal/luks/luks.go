package luks

import (
	"fmt"
	"os"
	"os/exec"
)

// CreateLUKSVolume set up a new LUKS volume with the specified size and password
func CreateLUKSVolume(filePath, password string, sizeMB int, useTPM bool) error {
	if sizeMB < 1 || sizeMB > 10 {
		return fmt.Errorf("size must be between 1MB and 10MB")
	}

	// Create a sparse file of the specified size
	if err := createSparseFile(filePath, sizeMB); err != nil {
		return fmt.Errorf("failed to create sparse file: %w", err)
	}

	// Optionally store the password in the TPM
	if useTPM {
		if err := storePasswordInTPM(password); err != nil {
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
	cmd.Stdin = createPasswordInput(password)
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
func CleanupLUKSVolume(mapperName, mountPoint string) error {
	fmt.Println("Unmounting LUKS volume...")
	if err := unmountLUKSVolume(mountPoint); err != nil {
		return fmt.Errorf("failed to unmount LUKS volume: %w", err)
	}

	fmt.Println("Closing LUKS volume...")
	if err := closeLUKSVolume(mapperName); err != nil {
		return fmt.Errorf("failed to close LUKS volume: %w", err)
	}

	fmt.Println("Removing mount directory...")
	if err := os.RemoveAll(mountPoint); err != nil {
		return fmt.Errorf("failed to remove mount directory: %w", err)
	}

	return nil
}

// MountLUKSVolume mounts the mapped LUKS volume to the specified mount point
func MountLUKSVolume(mapperName, mountPoint string) error {
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
	cmd.Stdin = createPasswordInput(password)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to format LUKS volume: %s", output)
	}
	return nil
}

// createPasswordInput creates a byte slice containing the password
func createPasswordInput(password string) *os.File {
	r, w, _ := os.Pipe()

	go func() {
		defer w.Close()
		w.WriteString(password + "\n")
	}()

	return r
}

// storePasswordInTPM stores the LUKS password securely in the TPM.
func storePasswordInTPM(password string) error {
	// Define the NV index
	cmd := exec.Command("tpm2_nvdefine",
		"0x1500016",
		"--size=64",
		"--attributes=ownerread|ownerwrite|authread|authwrite")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tpm2_nvdefine error: %s", string(output))
	}

	// Write the password to the NV index
	cmd = exec.Command("tpm2_nvwrite",
		"0x1500016",
		"--input=-")
	cmd.Stdin = createPasswordInput(password)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tpm2_nvwrite error: %s", string(output))
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
