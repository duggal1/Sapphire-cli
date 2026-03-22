package codeindex

import (
	"context"
	"testing"
)

func TestBundledQdrantBinaryDisabled(t *testing.T) {
	name, raw, err := bundledQdrantBinary()
	if err == nil {
		t.Fatal("expected bundled qdrant binary lookup to be disabled")
	}
	if name != "" {
		t.Fatalf("expected no binary name, got %q", name)
	}
	if raw != nil {
		t.Fatal("expected no bundled binary bytes")
	}
}

func TestBundledQdrantRuntimeStartDisabled(t *testing.T) {
	runtime := &qdrantRuntime{storageDir: t.TempDir(), baseURL: defaultQdrantURL}
	if err := runtime.Start(context.Background()); err == nil {
		t.Fatal("expected bundled qdrant runtime start to be disabled")
	}
}
