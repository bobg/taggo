package taggo

import (
	"fmt"
	"io"

	"github.com/bobg/modver/v2"
)

// Result holds the results of a call to [Check].
type Result struct {
	// DefaultBranch is the name of the default branch of the repository, typically "main" or "master".
	// This is determined heuristically from the repository's remote refs.
	DefaultBranch string

	// LatestVersion is the highest semantic version tag in the repository.
	LatestVersion string

	// LatestCommit is the hash of the latest commit on the main branch.
	// Valid only when DefaultBranch is not empty.
	LatestCommit string

	// LatestCommitHasLatestVersion is true if the latest commit on the main branch is tagged with the highest semantic version.
	// Valid only when DefaultBranch is not empty.
	LatestCommitHasLatestVersion bool

	// LatestCommitHasVersionTag is true if the latest commit on the main branch is tagged with any semantic version.
	// Valid only when DefaultBranch is not empty.
	LatestCommitHasVersionTag bool

	// LatestMajor, LatestMinor, LatestPatch are the major, minor, and patch components of the latest version tag.
	// Valid only when LatestVersion is not empty.
	LatestMajor, LatestMinor, LatestPatch int

	// LatestVersionIsPrerelease is true if the latest version tag is a prerelease.
	// Valid only when LatestVersion is not empty.
	LatestVersionIsPrerelease bool

	// LatestVersionUnstable is true if the latest version tag is unstable.
	// (I.e., the major version number is 0, or it is a prerelease.)
	// Valid only when LatestVersion is not empty.
	LatestVersionUnstable bool

	// Modpath is the import path of the Go module.
	Modpath string

	// ModpathMismatch is true if the trailing part of Modpath
	// (excluding any version suffix)
	// does not agree with ModuleSubdir.
	// In other words, if the module is in subdir foo/bar of its repository,
	// we'd expect Modpath to end with .../foo/bar.
	ModpathMismatch bool

	// ModuleSubdir is the subdir in the repository where the module lives.
	ModuleSubdir string

	// ModverResultCode is the result of a call to [modver.CompareGit]
	// on the latest tagged version and the latest commit on the main branch,
	// when those are different commits.
	// Valid only when DefaultBranch is not empty and LatestCommitHasVersionTag is false.
	ModverResultCode modver.ResultCode

	// ModverResultString is the string describing the result in ModverResultCode.
	// Valid only when DefaultBranch is not empty and LatestCommitHasVersionTag is false.
	ModverResultString string

	// NewMajor, NewMinor, NewPatch are the major, minor, and patch components of the recommended new version.
	// Valid when DefaultBranch is not empty and LatestCommitHasVersionTag is false,
	// or when there are not yet any version tags
	// (in which case the recommended new version is v0.1.0).
	NewMajor, NewMinor, NewPatch int

	// VersionPrefix is the prefix for version tags in the repository.
	// When the root of a Go module is in subdir foo/bar of its repository,
	// version tags must look like "foo/bar/v1.2.3";
	// this field holds the "foo/bar/" part.
	VersionPrefix string

	// VersionSuffix is the status of the module path's version suffix.
	// Valid only when LatestVersion is not empty.
	//
	// Possible values are:
	//
	//   - VSOK: the version suffix is required and present, and matches the major version of the latest version tag
	//   - VSMismatch: the version suffix does not match the major version of the latest version tag
	//   - VSMissing: a version suffix is required but missing
	//   - VSUnwanted: a version suffix is present but not required
	VersionSuffix VersionSuffixStatus
}

// VersionSuffixStatus is a type for the possible values of Result.VersionSuffix.
type VersionSuffixStatus string

// Possible values for Result.VersionSuffix.
const (
	VSOK       VersionSuffixStatus = "ok"
	VSMismatch VersionSuffixStatus = "mismatch"
	VSMissing  VersionSuffixStatus = "missing"
	VSUnwanted VersionSuffixStatus = "unwanted"
)

// Describe writes a human-readable description of r to w.
// If quiet is true, the description omits all but the warnings from the output, if any.
// The return value is the number of warnings emitted.
func (r Result) Describe(w io.Writer, quiet bool) int {
	var warnings int

	warnf := func(format string, args ...any) {
		warnings++
		showf(w, "⛔️", format, args...)
	}

	var (
		infof = func(_ string, _ ...any) {}
		okf   = func(_ string, _ ...any) {}
	)
	if !quiet {
		infof = func(format string, args ...any) {
			showf(w, "ℹ️", format, args...)
		}
		okf = func(format string, args ...any) {
			showf(w, "✅", format, args...)
		}
	}

	infof("Module path: %s", r.Modpath)
	if r.VersionPrefix != "" {
		infof("Version prefix: %s (n.b., this prefix is stripped from version tags appearing in this report)", r.VersionPrefix)
	}

	if r.DefaultBranch != "" {
		okf("Default branch: %s", r.DefaultBranch)
		infof("Latest commit hash: %s", r.LatestCommit)
	} else {
		warnf("Could not determine default branch")
	}

	if r.LatestVersion != "" {
		okf("Latest version tag: %s", r.LatestVersion)

		if r.LatestVersionIsPrerelease {
			warnf("Latest version %s is a prerelease", r.LatestVersion)
		} else {
			okf("Latest version %s is not a prerelease", r.LatestVersion)
		}

		if r.LatestVersionUnstable {
			warnf("Latest version %s is unstable", r.LatestVersion)
		} else {
			okf("Latest version %s is stable", r.LatestVersion)
		}

		switch r.VersionSuffix {
		case VSOK:
			if r.LatestMajor > 1 {
				okf("Module path %s has suffix matching major version %d", r.Modpath, r.LatestMajor)
			} else {
				okf("Module path %s neither needs nor has a version suffix", r.Modpath)
			}
		case VSMismatch:
			warnf("Module path %s version suffix does not agree with latest version %s", r.Modpath, r.LatestVersion)
		case VSMissing:
			warnf("Module path %s lacks suffix matching major version %d", r.Modpath, r.LatestMajor)
		case VSUnwanted:
			warnf("Module path %s contains an unwanted version suffix", r.Modpath)
		}

		if r.DefaultBranch != "" {
			if r.LatestCommitHasVersionTag {
				if r.LatestCommitHasLatestVersion {
					okf("Latest commit on the default branch has latest version tag")
				} else {
					warnf("Latest commit on the default branch has version tag, but it is not latest version %s", r.LatestVersion)
				}
			} else {
				warnf("Latest commit on the default branch lacks version tag")

				if r.ModverResultCode == modver.None {
					okf("Modver analysis: no new version tag required")
				} else {
					warnf("Modver analysis: %s", r.ModverResultString)
					warnf("Recommended new version: %sv%d.%d.%d", r.VersionPrefix, r.NewMajor, r.NewMinor, r.NewPatch)
					if r.NewMajor > r.LatestMajor && r.NewMajor > 1 {
						warnf("Module path will require new version suffix /v%d", r.NewMajor)
					}
				}
			}
		}
	} else {
		warnf("No version tags")
	}

	if r.ModpathMismatch {
		warnf("Module path %s does not agree with module subdir in repository %s", r.Modpath, r.ModuleSubdir)
	} else if r.ModuleSubdir != "" {
		okf("Module path %s agrees with module subdir in repository %s", r.Modpath, r.ModuleSubdir)
	}

	return warnings
}

func showf(w io.Writer, prefix, format string, args ...interface{}) {
	fmt.Fprint(w, prefix)
	fmt.Fprint(w, " ")
	fmt.Fprintf(w, format, args...)
	fmt.Fprintln(w)
}
