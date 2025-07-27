package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Custom error types
type EnvVarError struct {
	VarName string
	Err     error
}

func (e EnvVarError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("environment variable %s: %v", e.VarName, e.Err)
	}
	return fmt.Sprintf("environment variable %s is not set", e.VarName)
}

// ErrMissingEnvVar creates an error for a missing environment variable
func ErrMissingEnvVar(varName string) error {
	return EnvVarError{VarName: varName}
}

// getEnvOrDefault returns the value of an environment variable or a default value if not set
func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// GetRequiredEnvVar returns the value of a required environment variable or an error if not set.
func GetRequiredEnvVar(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", ErrMissingEnvVar(key)
	}
	return value, nil
}

// ReadJSONFile reads and unmarshals a JSON file into a target interface
func ReadJSONFile(filePath string, target interface{}) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File not existing is not an error here
		}
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal data from %s: %w", filePath, err)
	}

	return nil
}

// WriteJSONFile marshals and writes data to a JSON file, creating directories if needed
func WriteJSONFile(filePath string, data interface{}) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal data
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data for %s: %w", filePath, err)
	}

	// Write file
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
}
