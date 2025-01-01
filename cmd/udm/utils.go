package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

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
