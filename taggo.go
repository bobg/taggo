package taggo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/bobg/errors"
	"github.com/bobg/go-generics/v3/maps"
	"github.com/bobg/go-generics/v3/set"
	"github.com/bobg/modules"
	"github.com/bobg/modver/v2"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

// CheckAll calls [Check] on each Go module in a Git repository.
// It returns a map from module directory to the Result for that module.
// The git argument is the path to the git executable.
// If it is empty, [CheckAll] will look for "git" in PATH using [exec.LookPath].
func CheckAll(ctx context.Context, git, repodir string) (map[string]Result, error) {
	if git == "" {
		var err error
		git, err = exec.LookPath("git")
		if err != nil {
			return nil, errors.Wrap(err, "finding git binary")
		}
	}

	result := make(map[string]Result)
	err := modules.Each(repodir, func(moduledir string) error {
		res, err := Check(ctx, git, repodir, moduledir)
		if err == nil { // sic
			result[moduledir] = res
		}
		return err
	})
	return result, err
}

// Check checks a Go module in a Git repository.
// It returns a Result with information about the module and its repository.
func Check(ctx context.Context, git, repodir, moduledir string) (Result, error) {
	var result Result

	if git == "" {
		var err error
		git, err = exec.LookPath("git")
		if err != nil {
			return result, errors.Wrap(err, "finding git binary")
		}
	}

	if moduledir != "" {
		repodir = filepath.Clean(repodir)
		moduledir = filepath.Clean(moduledir)

		switch {
		case moduledir == repodir:
			moduledir = ""

		case filepath.IsAbs(moduledir):
			rel, err := filepath.Rel(repodir, moduledir)
			if err != nil {
				return result, errors.Wrapf(err, "finding relative path from %s to %s", repodir, moduledir)
			}
			if rel == ".." || strings.HasPrefix(rel, "../") {
				return result, fmt.Errorf("module dir %s is not in repository %s", moduledir, repodir)
			}
			if rel == "." {
				moduledir = ""
			} else {
				moduledir = rel
			}

		default:
			moduledir = strings.TrimPrefix(moduledir, repodir+"/")
		}
	}
	result.ModuleSubdir = moduledir

	var versionPrefix string
	if moduledir != "" {
		versionPrefix = moduledir + "/"
	}
	result.VersionPrefix = versionPrefix

	var (
		heads    = make(map[string]string)
		remotes  = make(map[string]map[string]string) // remote -> ref -> hash
		tags     = make(map[string]string)
		versions = make(map[string]string)
	)

	err := gitRefs(ctx, git, repodir, func(name, hash string) error {
		switch {
		case strings.HasPrefix(name, "refs/heads/"):
			name = strings.TrimPrefix(name, "refs/heads/")
			heads[name] = hash

		case strings.HasPrefix(name, "refs/remotes/"):
			name = strings.TrimPrefix(name, "refs/remotes/")

			parts := strings.SplitN(name, "/", 2)
			if len(parts) != 2 {
				return nil
			}
			remote, remoteRef := parts[0], parts[1]

			m, ok := remotes[remote]
			if !ok {
				m = make(map[string]string)
			}
			m[remoteRef] = hash
			remotes[remote] = m

		case strings.HasPrefix(name, "refs/tags/"):
			name = strings.TrimPrefix(name, "refs/tags/")

			// Extra step to resolve the tag's underlying commit,
			// if it's an annotated tag.
			hash, err := gitTagCommit(ctx, git, repodir, name)
			if err != nil {
				return errors.Wrapf(err, "resolving commit for tag %s", name)
			}

			tags[name] = hash

			if versionPrefix != "" {
				if !strings.HasPrefix(name, versionPrefix) {
					return nil
				}
				name = strings.TrimPrefix(name, versionPrefix)
			}
			if semver.IsValid(name) {
				versions[name] = hash
			}
		}
		return nil
	})
	if err != nil {
		return result, errors.Wrap(err, "getting refs")
	}

	var (
		latestVersion                         string
		latestMajor, latestMinor, latestPatch int // valid only if latestVersion is non-empty
		latestVersionUnstable                 bool
		latestVersionIsPrerelease             bool
		versionTags                           = maps.Keys(versions)
	)
	semver.Sort(versionTags)
	if len(versionTags) > 0 {
		latestVersion = versionTags[len(versionTags)-1]

		m := versionRegex.FindStringSubmatch(latestVersion)
		if len(m) == 0 {
			return result, fmt.Errorf("parsing version %s", latestVersion)
		}
		latestMajor, _ = strconv.Atoi(m[1])
		latestMinor, _ = strconv.Atoi(m[2])
		latestPatch, _ = strconv.Atoi(m[3])

		latestVersionIsPrerelease = semver.Prerelease(latestVersion) != ""
		latestVersionUnstable = latestMajor == 0 || latestVersionIsPrerelease
	}
	result.LatestVersion = latestVersion
	result.LatestMajor = latestMajor
	result.LatestMinor = latestMinor
	result.LatestPatch = latestPatch
	result.LatestVersionIsPrerelease = latestVersionIsPrerelease
	result.LatestVersionUnstable = latestVersionUnstable

	gomodPath := filepath.Join(repodir, moduledir, "go.mod")
	gomodBytes, err := os.ReadFile(gomodPath)
	if err != nil {
		return result, errors.Wrapf(err, "reading %s", gomodPath)
	}
	gomod, err := modfile.ParseLax(gomodPath, gomodBytes, noopFixer)
	if err != nil {
		return result, errors.Wrapf(err, "parsing %s", gomodPath)
	}

	result.Modpath = gomod.Module.Mod.Path
	result.VersionSuffix = VSOK

	baseModpath, modpathSuffixVersion, hasModpathVersionSuffix := decomposeModpath(gomod.Module.Mod.Path)
	if hasModpathVersionSuffix {
		switch modpathSuffixVersion {
		case 0, 1:
			result.VersionSuffix = VSUnwanted

		case latestMajor:
			// ok, do nothing

		default:
			result.VersionSuffix = VSMismatch
		}
	} else if latestMajor > 1 {
		result.VersionSuffix = VSMissing
	}

	if moduledir != "" {
		suffix := "/" + moduledir
		if !strings.HasSuffix(baseModpath, suffix) {
			result.ModpathMismatch = true
		}
	}

	defaultBranch := detectDefaultBranch(remotes["origin"], heads)
	if defaultBranch == "" {
		for _, remoteRefs := range remotes {
			if defaultBranch = detectDefaultBranch(remoteRefs, heads); defaultBranch != "" {
				break
			}
		}
	}
	result.DefaultBranch = defaultBranch

	var latestCommitHasVersionTag bool

	if defaultBranch != "" {
		if latestCommit, ok := heads[defaultBranch]; ok {
			latestCommitHasLatestVersion := versions[latestVersion] == latestCommit

			latestCommitHasVersionTag = latestCommitHasLatestVersion
			if !latestCommitHasVersionTag {
				for _, h := range versions {
					if h == latestCommit {
						latestCommitHasVersionTag = true
						break
					}
				}
			}

			result.LatestCommit = latestCommit
			result.LatestCommitHasVersionTag = latestCommitHasVersionTag
			result.LatestCommitHasLatestVersion = latestCommitHasLatestVersion
		}
	}

	var newMajor, newMinor, newPatch int

	if latestVersion != "" {
		if defaultBranch != "" && !latestCommitHasVersionTag {
			latestVersionWithPrefix := versionPrefix + latestVersion

			newMajor, newMinor, newPatch = latestMajor, latestMinor, latestPatch

			ctx = modver.WithGit(ctx, git)

			dotgitdir := filepath.Join(repodir, ".git")
			modverResult, err := modver.CompareGit(ctx, dotgitdir, latestVersionWithPrefix, defaultBranch)
			if err != nil {
				return result, errors.Wrapf(err, "comparing %s to %s", latestVersionWithPrefix, defaultBranch)
			}
			result.ModverResultCode = modverResult.Code()
			result.ModverResultString = modverResult.String()

			switch modverResult.Code() {
			case modver.Major:
				newMajor, newMinor, newPatch = latestMajor+1, 0, 0

			case modver.Minor:
				newMajor, newMinor, newPatch = latestMajor, latestMinor+1, 0

			case modver.Patchlevel:
				if !latestVersionIsPrerelease {
					newPatch = latestPatch + 1
				}
			}
		}
	} else {
		newMajor, newMinor, newPatch = 0, 1, 0
	}
	result.NewMajor = newMajor
	result.NewMinor = newMinor
	result.NewPatch = newPatch

	return result, nil
}

func detectDefaultBranch(remoteRefs map[string]string, heads map[string]string) string {
	var (
		remoteNames = set.New(maps.Keys(remoteRefs)...)
		headNames   = set.New(maps.Keys(heads)...)
		candidates  = set.Intersect(remoteNames, headNames)
	)

	for _, name := range []string{"main", "master", "default"} {
		if candidates.Has(name) && remoteRefs[name] == heads[name] {
			return name
		}
	}

	if len(candidates) == 1 {
		slices := candidates.Slice()
		name := slices[0]
		if strings.ContainsFunc(name, nonDefaultBranchRune) {
			return ""
		}
		if remoteRefs[name] == heads[name] {
			return name
		}
	}

	return ""
}

func nonDefaultBranchRune(r rune) bool {
	return !unicode.IsOneOf([]*unicode.RangeTable{unicode.Letter, unicode.Digit}, r)
}

func noopFixer(_, version string) (string, error) {
	return version, nil
}

func decomposeModpath(modpath string) (baseModpath string, suffixVersion int, hasVersionSuffix bool) {
	if m := modpathVersionSuffixRegex.FindStringSubmatchIndex(modpath); len(m) > 0 {
		baseModpath = modpath[:m[2]]
		suffixVersion, _ = strconv.Atoi(modpath[m[2]:m[3]])
		return baseModpath, suffixVersion, true
	}
	return modpath, 0, false
}

var (
	modpathVersionSuffixRegex = regexp.MustCompile(`/v([1-9][0-9]*)$`)
	versionRegex              = regexp.MustCompile(`v([0-9]+)\.([0-9]+)\.([0-9]+)`)
)
