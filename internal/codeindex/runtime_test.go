package codeindex

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestBundledQdrantBinaryAvailableForCurrentPlatform(t *testing.T) {
	name, raw, err := bundledQdrantBinary()
	if err != nil {
		t.Fatalf("expected bundled binary for current platform: %v", err)
	}
	if name == "" {
		t.Fatal("expected bundled binary name")
	}
	if len(raw) == 0 {
		t.Fatal("expected bundled binary bytes")
	}
}

func TestBundledQdrantRuntimeStarts(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:"+defaultQdrantPort)
	if err != nil {
		t.Skip("default qdrant port is already in use")
	}
	_ = listener.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	workDir := t.TempDir()
	store, err := openStore(t.TempDir(), workDir, 8, defaultQdrantURL)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	if err := store.ensureRuntime(ctx); err != nil {
		t.Fatalf("start bundled qdrant: %v", err)
	}
	if err := store.ping(ctx); err != nil {
		t.Fatalf("ping bundled qdrant: %v", err)
	}
}
