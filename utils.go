package main

import (
	"fmt"
	"os"
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
