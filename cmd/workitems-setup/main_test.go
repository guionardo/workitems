package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteOutputProtectsExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider.env")
	if err := writeOutput(path, false, []string{"TOKEN="}); err != nil {
		t.Fatalf("writeOutput() error = %v", err)
	}
	if err := writeOutput(path, false, []string{"TOKEN=changed"}); err == nil {
		t.Fatal("writeOutput() succeeded, want existing-file error")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.TrimSpace(string(data)) != "TOKEN=" {
		t.Errorf("writeOutput() wrote %q, want TOKEN=", string(data))
	}
}
