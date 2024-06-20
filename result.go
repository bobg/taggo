package vtag

import (
	"fmt"
	"io"

	"github.com/bobg/modver/v2"
)

type Result struct {
	LatestCommitHasVersionTag             bool
	LatestHash                            string
	LatestMajor, LatestMinor, LatestPatch int
	LatestVersion                         string
	LatestVersionIsPrerelease             bool
	LatestVersionUnstable                 bool
	MainBranch                            string
	MissingVersionPrefix                  bool
	MissingVersionSuffix                  bool
	Modpath                               string
	ModverResult                          modver.Result
	NewMajor, NewMinor, NewPatch          int
	NewModpath                            string
	NoVersions                            bool
	UnwantedVersionSuffix                 bool
	VersionPrefix                         string
	VersionSuffixMismatch                 bool
}

func (r Result) Describe(w io.Writer, quiet bool) {
	okf(w, quiet, "Module path: %s", r.Modpath)
	if r.LatestVersion != "" {
		okf(w, quiet, "Latest version tag: %s", r.LatestVersion)
	}

	if r.LatestCommitHasVersionTag {
		okf(w, quiet, "Latest commit has version tag")
	} else {
		warnf(w, "Latest commit lacks version tag")

		if r.ModverResult != nil {
			if r.ModverResult.Code() == modver.None {
				okf(w, quiet, "No version change required")
			} else {
				warnf(w, "Recommended new version: v%d.%d.%d (based on Modver analysis: %s)", r.NewMajor, r.NewMinor, r.NewPatch, r.ModverResult)
				if r.NewModpath != "" {
					warnf(w, "Recommended new module path: %s", r.NewModpath)
				}
			}
		}
	}

	if r.NoVersions {
		warnf(w, "No version tags")
	} else {
		if r.LatestVersionIsPrerelease {
			warnf(w, "Latest version %s is a prerelease", r.LatestVersion)
		}
		if r.LatestVersionUnstable {
			warnf(w, "Latest version %s is unstable", r.LatestVersion)
		}
		if r.MissingVersionPrefix {
			warnf(w, "One or more version tags lack prefix %s", r.VersionPrefix)
		}
		if r.MissingVersionSuffix {
			warnf(w, "Module path %s lacks suffix matching major version %d", r.Modpath, r.LatestMajor)
		}
	}

	if r.UnwantedVersionSuffix {
		warnf(w, "Module path %s contains an unwanted version suffix", r.Modpath)
	}

	if r.VersionSuffixMismatch {
		warnf(w, "Module path %s has a version suffix that does not match the major version of latest version tag %s", r.Modpath, r.LatestVersion)
	}
}

func warnf(w io.Writer, format string, args ...interface{}) {
	fmt.Fprintf(w, "⚠️ ")
	fmt.Fprintf(w, format, args...)
	fmt.Fprintln(w) // xxx
}

func okf(w io.Writer, quiet bool, format string, args ...interface{}) {
	if quiet {
		return
	}
	fmt.Fprintf(w, "✅ ")
	fmt.Fprintf(w, format, args...)
	fmt.Fprintln(w) // xxx
}
