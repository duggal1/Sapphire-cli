package version

import "testing"

func TestDisplay(t *testing.T) {
	previous := Version
	t.Cleanup(func() {
		Version = previous
	})

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "stable", version: "v1.1.8", want: "1.1.8"},
		{name: "dev build", version: "1.1.7.dev.20260331182836", want: "1.1.7"},
		{name: "main build", version: "1.1.8.main.386.20260401003950", want: "1.1.8"},
		{name: "dot nightly", version: "1.1.8.nightly.386.6364d79", want: "1.1.8 nightly"},
		{name: "nightly", version: "1.1.7-nightly-a061aaf", want: "1.1.7 nightly"},
		{name: "blank", version: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version = tt.version
			if got := Display(); got != tt.want {
				t.Fatalf("Display() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCLI(t *testing.T) {
	previous := Version
	t.Cleanup(func() {
		Version = previous
	})

	Version = "v1.1.7.main.421.20260517194324"
	if got := CLI(); got != "1.1.7.main.421.20260517194324" {
		t.Fatalf("CLI() = %q, want %q", got, "1.1.7.main.421.20260517194324")
	}
}
