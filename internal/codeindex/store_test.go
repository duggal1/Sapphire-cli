package codeindex

import "testing"

func TestShouldManageLocalQdrant(t *testing.T) {
	cases := []struct {
		name string
		base string
		raw  string
		want bool
	}{
		{name: "default local no config", base: defaultQdrantURL, raw: "", want: true},
		{name: "explicit localhost", base: "http://localhost:6333", raw: "http://localhost:6333", want: true},
		{name: "explicit loopback", base: "http://127.0.0.1:6333", raw: "http://127.0.0.1:6333", want: true},
		{name: "custom remote", base: "https://qdrant.example.com", raw: "https://qdrant.example.com", want: false},
		{name: "custom local port", base: "http://localhost:7000", raw: "http://localhost:7000", want: false},
	}
	for _, tc := range cases {
		if got := shouldManageLocalQdrant(tc.base, tc.raw); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestPointsFromFileSkipsChunksWithoutEmbeddings(t *testing.T) {
	file := indexedFile{
		ContentHash: "hash",
		Chunks: []indexedChunk{
			{ID: "a", Embedding: []float32{1, 2}, Path: "a.go"},
			{ID: "b", Embedding: nil, Path: "a.go"},
		},
	}
	points := pointsFromFile(file)
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
}
