package codeindex

import (
	"regexp"
	"testing"
)

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

func TestChunkFileUsesSingleChunkForSmallFiles(t *testing.T) {
	file := discoveredFile{
		RelativePath: "internal/example.ts",
		Language:     "typescript",
		Content:      "export function add(a: number, b: number) { return a + b }\n",
		ContentHash:  hashBytes([]byte("export function add(a: number, b: number) { return a + b }\n")),
		Size:         int64(len("export function add(a: number, b: number) { return a + b }\n")),
	}
	chunks := chunkFile(file)
	if len(chunks) != 1 {
		t.Fatalf("expected single chunk for small file, got %d", len(chunks))
	}
	if chunks[0].Kind != "file" {
		t.Fatalf("expected file chunk kind, got %q", chunks[0].Kind)
	}
}

func TestSanitizeTextMakesInvalidUTF8Safe(t *testing.T) {
	raw := []byte{'a', 0xff, 'b', '\x1b', '[', '0', 'm'}
	got := sanitizeText(raw)
	if got == "" {
		t.Fatal("expected sanitized text")
	}
	for _, r := range got {
		if r == '\x1b' {
			t.Fatalf("expected control characters to be removed, got %q", got)
		}
	}
}
