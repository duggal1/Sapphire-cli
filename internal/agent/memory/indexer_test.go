package memory

import "testing"

func TestRecommendedIndexedRepoWorkersScalesForLargeRepos(t *testing.T) {
	t.Parallel()

	if got := recommendedIndexedRepoWorkers(0); got != 0 {
		t.Fatalf("workers for zero files = %d, want 0", got)
	}
	if got := recommendedIndexedRepoWorkers(2); got < 2 {
		t.Fatalf("workers for small repo = %d, want at least 2", got)
	}
	if got := recommendedIndexedRepoWorkers(5000); got < 4 {
		t.Fatalf("workers for large repo = %d, want greater parallelism", got)
	}
	if got := recommendedIndexedRepoWorkers(5000); got > 16 {
		t.Fatalf("workers for large repo = %d, want bounded parallelism", got)
	}
}
