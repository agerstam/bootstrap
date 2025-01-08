package luks

import (
	"os"
	"os/exec"
	"testing"
)

func TestCreateLUKSVolume(t *testing.T) {
	testFile := "test-luks-volume.img"
	password := []byte("MyStr0ngP@ssw0rd!")
	sizeMB := 5
	useTPM := false

	defer os.Remove(testFile)

	if err := CreateLUKSVolume(testFile, password, sizeMB, useTPM); err != nil {
		t.Fatalf("CreateLUKSVolume() error = %v, want nil", err)
	}

	// Verify the file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatalf("LUKS volumefile does not exist: %v", testFile)
	}
}
func TestCreateLUKSVolumeWithTPM(t *testing.T) {
	testFile := "test-luks-volume-with-tpm.img"
	password := []byte("MyStr0ngP@ssw0rd!")
	sizeMB := 5
	useTPM := true

	defer os.Remove(testFile)

	// Mock TPM availability check if necessary
	if !isTPMAvailable() {
		t.Skip("Skipping test: TPM not available on this system")
	}

	if err := CreateLUKSVolume(testFile, password, sizeMB, useTPM); err != nil {
		t.Fatalf("Failed to create LUKS volume with TPM: %v", err)
	}

	// Verify the file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatalf("LUKS volume file does not exist: %v", testFile)
	}
}

// isTPMAvailable checks if the TPM is available on the system.
func isTPMAvailable() bool {
	// Check if TPM is accessible using tpm2-tools
	cmd := exec.Command("tpm2_getcap", "properties-fixed")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
