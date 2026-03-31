package version

import "strings"

// Build-time parameters set via -ldflags

// Version is the current Sapphire version.
var Version = "v1.1.8"

// Display returns a simplified version string for UI surfaces.
func Display() string {
	v := strings.TrimSpace(Version)
	v = strings.TrimPrefix(v, "v")
	switch {
	case v == "":
		return ""
	case strings.Contains(v, ".dev."):
		return strings.SplitN(v, ".dev.", 2)[0]
	case strings.Contains(v, ".main."):
		return strings.SplitN(v, ".main.", 2)[0]
	case strings.Contains(v, ".nightly."):
		return strings.SplitN(v, ".nightly.", 2)[0] + " nightly"
	case strings.Contains(v, "-nightly-"):
		return strings.SplitN(v, "-nightly-", 2)[0] + " nightly"
	default:
		return v
	}
}
