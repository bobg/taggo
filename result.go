package vtag

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

	// LatestCommitHasLatestVersion is true if the latest commit on the main branch is tagged with the highest semantic version.
	// Valid only when DefaultBranch is not empty.
	LatestCommitHasLatestVersion bool

	// LatestCommitHasVersionTag is true if the latest commit on the main branch is tagged with any semantic version.
	// Valid only when DefaultBranch is not empty.
	LatestCommitHasVersionTag bool

	// LatestHash is the hash of the latest commit on the main branch.
	// Valid only when DefaultBranch is not empty.
	LatestHash string

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
	//   - "ok": the version suffix is required and present, and matches the major version of the latest version tag
	//   - "mismatch": the version suffix does not match the major version of the latest version tag
	//   - "missing": a version suffix is required but missing
	//   - "unwanted": a version suffix is present but not required
	VersionSuffix VersionSuffixStatus
}

type VersionSuffixStatus string

const (
	VSOK       VersionSuffixStatus = "ok"
	VSMismatch VersionSuffixStatus = "mismatch"
	VSMissing  VersionSuffixStatus = "missing"
	VSUnwanted VersionSuffixStatus = "unwanted"
)

func (r Result) Describe(w io.Writer, quiet bool) {
	infof(w, quiet, "Module path: %s", r.Modpath)
	if r.VersionPrefix != "" {
		infof(w, quiet, "Version prefix: %s (n.b., this prefix is stripped from version tags appearing in this report)", r.VersionPrefix)
	}

	if r.DefaultBranch != "" {
		okf(w, quiet, "Default branch: %s", r.DefaultBranch)
		infof(w, quiet, "Latest commit hash: %s", r.LatestHash)
	} else {
		warnf(w, "Could not determine default branch")
	}

	if r.LatestVersion != "" {
		okf(w, quiet, "Latest version tag: %s", r.LatestVersion)

		if r.LatestVersionIsPrerelease {
			warnf(w, "Latest version %s is a prerelease", r.LatestVersion)
		} else {
			okf(w, quiet, "Latest version %s is not a prerelease", r.LatestVersion)
		}

		if r.LatestVersionUnstable {
			warnf(w, "Latest version %s is unstable", r.LatestVersion)
		} else {
			okf(w, quiet, "Latest version %s is stable", r.LatestVersion)
		}

		switch r.VersionSuffix {
		case VSOK:
			if r.LatestMajor > 1 {
				okf(w, quiet, "Module path %s has suffix matching major version %d", r.Modpath, r.LatestMajor)
			} else {
				okf(w, quiet, "Module path %s neither needs nor has a version suffix", r.Modpath)
			}
		case VSMismatch:
			warnf(w, "Module path %s version suffix does not agree with latest version %s", r.Modpath, r.LatestVersion)
		case VSMissing:
			warnf(w, "Module path %s lacks suffix matching major version %d", r.Modpath, r.LatestMajor)
		case VSUnwanted:
			warnf(w, "Module path %s contains an unwanted version suffix", r.Modpath)
		}

		if r.DefaultBranch != "" {
			if r.LatestCommitHasVersionTag {
				if r.LatestCommitHasLatestVersion {
					okf(w, quiet, "Latest commit on the default branch has latest version tag")
				} else {
					warnf(w, "Latest commit on the default branch has version tag, but it is not latest version %s", r.LatestVersion)
				}
			} else {
				warnf(w, "Latest commit on the default branch lacks version tag")

				if r.ModverResultCode == modver.None {
					okf(w, quiet, "Modver analysis: no new version tag required")
				} else {
					warnf(w, "Modver analysis: %s", r.ModverResultString)
					warnf(w, "Recommended new version: %sv%d.%d.%d", r.VersionPrefix, r.NewMajor, r.NewMinor, r.NewPatch)
					if r.NewMajor > r.LatestMajor && r.NewMajor > 1 {
						warnf(w, "Module path will require new version suffix /v%d", r.NewMajor)
					}
				}
			}
		}
	} else {
		warnf(w, "No version tags")
	}

	if r.ModpathMismatch {
		warnf(w, "Module path %s does not agree with module subdir in repository %s", r.Modpath, r.ModuleSubdir)
	} else if r.ModuleSubdir != "" {
		okf(w, quiet, "Module path %s agrees with module subdir in repository %s", r.Modpath, r.ModuleSubdir)
	}
}

func warnf(w io.Writer, format string, args ...any) {
	showf(w, false, "⛔️", format, args...)
}

func okf(w io.Writer, quiet bool, format string, args ...any) {
	showf(w, quiet, "✅", format, args...)
}

func infof(w io.Writer, quiet bool, format string, args ...any) {
	showf(w, quiet, "ℹ️", format, args...)
}

func showf(w io.Writer, quiet bool, prefix, format string, args ...interface{}) {
	if quiet {
		return
	}

	fmt.Fprint(w, prefix)
	fmt.Fprint(w, " ")
	fmt.Fprintf(w, format, args...)
	fmt.Fprintln(w)
}
