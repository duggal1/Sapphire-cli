package contextmanager

const (
	LargeContextWindowThreshold = 200_000
	LargeContextWindowBuffer    = 30_000
	SmallContextWindowRatio     = 0.15
	SmallContextWindowMinBuffer = 3_000
)

func CompactionThreshold(contextWindow int64) int64 {
	var threshold int64
	if contextWindow > LargeContextWindowThreshold {
		threshold = LargeContextWindowBuffer
	} else {
		threshold = int64(float64(contextWindow) * SmallContextWindowRatio)
	}
	if threshold < SmallContextWindowMinBuffer {
		threshold = SmallContextWindowMinBuffer
	}
	return threshold
}

func ShouldCompact(contextWindow, promptTokens, completionTokens int64) bool {
	if contextWindow <= 0 {
		return false
	}
	remaining := contextWindow - (promptTokens + completionTokens)
	return remaining <= CompactionThreshold(contextWindow)
}
