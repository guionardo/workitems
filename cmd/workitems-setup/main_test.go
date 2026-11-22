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

func TestOwnerTypes(t *testing.T) {
	tests := []struct {
		name string
		kind string
		want string
	}{
		{name: "auto", kind: "auto", want: "user,organization"},
		{name: "user", kind: "user", want: "user"},
		{name: "organization", kind: "organization", want: "organization"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(ownerTypes(test.kind), ",")
			if got != test.want {
				t.Errorf("ownerTypes(%q) = %q, want %q", test.kind, got, test.want)
			}
		})
	}
}
