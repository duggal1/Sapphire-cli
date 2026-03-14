package memory

import (
	"context"
	"strings"
)

// FallbackExtractor is a local heuristic extractor used when the primary model fails.
type FallbackExtractor struct{}

// NewFallbackExtractor returns a lightweight extractor with no external dependencies.
func NewFallbackExtractor() *FallbackExtractor {
	return &FallbackExtractor{}
}

func (f *FallbackExtractor) Name() string { return "local-fallback" }

// Extract applies minimal heuristics to preserve key signals without model calls.
func (f *FallbackExtractor) Extract(_ context.Context, rawSource string) (*ExtractionResult, error) {
	result := &ExtractionResult{}
	lines := strings.Split(rawSource, "\n")

	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		if lower == "" {
			continue
		}

		if containsAny(lower, []string{"error", "failed", "panic", "exception"}) {
			result.FailuresEncountered = append(result.FailuresEncountered, FailureEncountered{
				WhatFailed: line,
			})
		}

		if containsAny(lower, []string{"do not", "don't", "never", "avoid"}) {
			result.NegativeConstraints = append(result.NegativeConstraints, NegativeConstraint{
				Constraint: line,
				Reason:     "captured from fallback extractor",
			})
		}
	}

	return result, nil
}

func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}
