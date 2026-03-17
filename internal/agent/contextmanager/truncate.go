package contextmanager

func ApproxBytesForTokens(tokens int) int {
	if tokens <= 0 {
		return 0
	}
	return tokens * 4
}
