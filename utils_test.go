package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestGetRequiredEnvVar(t *testing.T) {
	const key = "TEST_REQUIRED_ENV_VAR"
	_ = os.Unsetenv(key)
	if _, err := GetRequiredEnvVar(key); err == nil {
		t.Fatalf("expected error for missing env var")
	}

	_ = os.Setenv(key, "value")
	t.Cleanup(func() { _ = os.Unsetenv(key) })
	val, err := GetRequiredEnvVar(key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "value" {
		t.Fatalf("unexpected value: %s", val)
	}
}

func TestReadWriteJSONFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "data.json")

	// Read non-existent file should be nil error
	var m map[string]string
	if err := ReadJSONFile(file, &m); err != nil {
		t.Fatalf("unexpected error reading non-existent file: %v", err)
	}

	// Write and read back
	input := map[string]string{"a": "b"}
	if err := WriteJSONFile(file, input); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Verify permissions are 0600
	info, err := os.Stat(file)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("unexpected file perms: %v", perm)
	}

	var out map[string]string
	if err := ReadJSONFile(file, &out); err != nil {
		t.Fatalf("read back failed: %v", err)
	}
	if out["a"] != "b" {
		t.Fatalf("unexpected content: %#v", out)
	}
}

func TestEnvVarErrorFormatting(t *testing.T) {
	// Missing var path (Err == nil)
	err := ErrMissingEnvVar("MY_ENV")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.As(err, &EnvVarError{}) {
		t.Fatalf("expected EnvVarError type")
	}
	if err.Error() != "environment variable MY_ENV is not set" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}

	// Wrapped error path
	e := EnvVarError{VarName: "OTHER", Err: errors.New("boom")}
	if e.Error() != "environment variable OTHER: boom" {
		t.Fatalf("unexpected wrapped error message: %q", e.Error())
	}
}
