package mock_up

import "testing"

func TestDouble(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{2, 4},
		{0, 0},
		{-5, -10},
	}

	for _, tt := range tests {
		result := Double(tt.input)
		if result != tt.expected {
			t.Errorf("Double(%d) = %d; want %d", tt.input, result, tt.expected)
		}
	}
}
