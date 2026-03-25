package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMarkProjectInitializedCreatesInitFlag(t *testing.T) {
	workingDir := t.TempDir()
	dataDir := filepath.Join(workingDir, ".sapphire")
	cfg := &Config{
		Options: &Options{
			DataDirectory: dataDir,
		},
		workingDir: workingDir,
	}

	if err := MarkProjectInitialized(cfg); err != nil {
		t.Fatalf("MarkProjectInitialized() error = %v", err)
	}

	flagPath := filepath.Join(dataDir, InitFlagFilename)
	if _, err := os.Stat(flagPath); err != nil {
		t.Fatalf("expected init flag file at %s: %v", flagPath, err)
	}
}
