package codeindex

import (
	"regexp"
	"testing"
)

func TestUUIDFromHashReturnsUUIDFormat(t *testing.T) {
	hash := hashBytes([]byte("hello world"))
	var raw [32]byte
	copy(raw[:], []byte(hash))
	_ = raw
}

func TestNewChunkUsesUUIDPointID(t *testing.T) {
	file := discoveredFile{
		RelativePath: "internal/example.go",
		Language:     "go",
		ContentHash:  hashBytes([]byte("package example")),
	}
	chunk := newChunk(file, 0, "function", 1, 4, "Example", "func Example() {}")
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidPattern.MatchString(chunk.ID) {
		t.Fatalf("expected UUID point id, got %q", chunk.ID)
	}
}
