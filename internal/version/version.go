package version

// These variables are overridden by the linker during a release build.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a human-readable version line.
func String() string {
	return "elostirion " + Version + " (commit " + Commit + ", built " + Date + ")"
}
