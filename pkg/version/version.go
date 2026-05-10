package version

import (
	"fmt"
	"io"
	"text/tabwriter"
)

var (
	// Tag is the tagged version, for example v0.1.0.
	Tag = "dev"
	// Time is the UTC build time.
	Time string
	// User is the user that built the binary.
	User string
	// Commit is the git commit hash used for the build.
	Commit string
)

// GetVersion returns the compact version string used by help and logs.
func GetVersion() string {
	return Tag + "-" + Time + ":" + User
}

// Print writes the full build version information.
func Print(w io.Writer) error {
	tw := new(tabwriter.Writer)
	tw.Init(w, 0, 0, 0, ' ', tabwriter.AlignRight)
	for _, line := range [][]any{
		nil,
		{"webcap:", "\t", "Browser screenshots, image diffs, workflow reports, and MCP tools"},
		{"Version:", "\t", Tag},
		{"Build Commit Hash:", "\t", Commit},
		{"Build Time:", "\t", Time},
		{"Build User:", "\t", User},
		{"Info:", "\t", "https://github.com/goliatone/webcap"},
		nil,
	} {
		if _, err := fmt.Fprintln(tw, line...); err != nil {
			return err
		}
	}
	return tw.Flush()
}
