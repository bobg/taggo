# Taggo

[![Go Reference](https://pkg.go.dev/badge/github.com/bobg/taggo.svg)](https://pkg.go.dev/github.com/bobg/taggo)
[![Go Report Card](https://goreportcard.com/badge/github.com/bobg/taggo)](https://goreportcard.com/report/github.com/bobg/taggo)

This is taggo,
a Go library and command
that analyzes one or more Go modules in a Git repository
to find problems in version tags and module paths.

## Installation

```sh
go install github.com/bobg/taggo/cmd/taggo@latest
```

## Usage

```sh
taggo [-all] [-git GIT] [-json] [-q] [-status] [REPODIR] [MODULEDIR]
```

Taggo requires two directories:
the root of a Git repository,
and the root of a Go module.
Often these are the same directory,
but not always.

If two directories are specified on the command line,
they are taken to be the repository root and the module root,
in that order.

If one directory is specified,
Taggo searches it and its parents for the nearest `go.mod` file
to determine the module root.
Taggo then searches that and _its_ parents for the nearest `.git` subdirectory
to determine the repository root.
If `-all` is specified,
only the repository root is sought.

If no directories are specified,
Taggo performs the same search beginning at the current directory.

Flags and their meanings are:

| Flag     | Meaning                                                                                                             |
|----------|---------------------------------------------------------------------------------------------------------------------|
| -add     | Add a new version tag, if recommended. Refuses if the repository is not clean or a new major version is needed.     |
| -all     | Check all Go modules in the repository.                                                                             |
| -git GIT | The path to the `git` binary, by default the result of [exec.LookPath](https://pkg.go.dev/os/exec#LookPath)("git"). |
| -json    | Output a JSON representation of the result (as a [taggo.Result](https://pkg.go.dev/github.com/bobg/taggo#Result)).  |
| -msg MSG | With -add, annotate the new tag with this message. By default it’s “Version ... added by Taggo.”                    |
| -q       | Suppress all output except for warnings.                                                                            |
| -s       | With -add, sign the new tag with GPG. See https://git-scm.com/docs/git-tag#Documentation/git-tag.txt--s.            |
| -status  | Exit with status 2 if any warnings are reported.                                                                    |

When `-add` refuses to add a tag because it would change the major version number,
it causes Taggo to exit with status 3.
If combined with `-status`
and any warnings are reported
(which, if a new version tag is recommended, they will be),
Taggo exits with status 6
(the product of 2×3).

## Findings

This section describes the different findings that Taggo may report.

### ℹ️ Module path: ...

The import path of the Go module.

### ℹ️ Version prefix: ...

The prefix required for version tags on this module.
If the module is at the repository root,
this is empty.
But if the module is in subdirectory `foo/bar`,
then version tags must begin with the string `foo/bar/`,
as in `foo/bar/v1.2.3`.
See [go.dev/ref/mod#vcs-version](https://go.dev/ref/mod#vcs-version).

Taggo strips off the version prefix when reporting most of these findings,
but includes it in “Recommended new version” (if present).

### ✅ Default branch: ...

The default branch name of the repository, usually `master` or `main`.
This is determined heuristically from the repository’s remote refs.

### ✅ Latest commit hash: ...

Git commit hash of the latest commit on the default branch,
if that branch could be determined.

### ⛔️ Could not determine default branch

The heuristic for determining the repository’s default branch failed.
Some findings will not be available as a result.

### ✅ Latest version tag: ...

The highest semantic version tag found
(after removing any required version prefix).

### ⛔️ Latest version ... is a prerelease

The latest version tag has a prerelease suffix.
Example: `v1.2.3-pre1`.
See [go.dev/ref/mod#glos-pre-release-version](https://go.dev/ref/mod#glos-pre-release-version).

### ✅ Latest version ... is not a prerelease

The latest version tag does not have a prerelease suffix.

### ⛔️ Latest version ... is unstable

The latest version tag is unstable:
either it is a prerelease,
or the major version number is zero.
“Unstable” means that callers have no expectation of compatibility
between this and any other version.
See [go.dev/ref/mod#versions](https://go.dev/ref/mod#versions).

### ✅ Latest version ... is stable

The latest version tag is stable:
it has no prerelease suffix,
and the major version number is 1 or higher.

### ⛔️ Module path ... lacks suffix matching major version ...

The module path requires a major-version suffix but does not have one.
This happens when the latest version has major version N,
where N is 2 or higher,
and the module path does not end with `/vN`.
See [go.dev/ref/mod#major-version-suffixes](https://go.dev/ref/mod#major-version-suffixes).

### ⛔️ Module path ... version suffix does not agree with latest version ...

The module path has a major-version suffix
that does not match the major version number of the latest version tag.

### ⛔️ Module path ... contains an unwanted version suffix

The module path should not have a major-version suffix
(because the latest version has major version number 0 or 1)
but has one anyway.

### ✅ Module path ... has suffix matching major version ...

The module path requires a major-version suffix and has the correct one.

### ✅ Module path ... neither needs nor has a version suffix

The module path does not require a major-version suffix and does not have one.

### ✅ Latest commit on the default branch has latest version tag

The latest commit on the default branch has a version tag,
and it’s the highest version tag found.

### ⛔️ Latest commit on the default branch has version tag, but it is not latest version ...

The latest commit on the default branch has a version tag,
but it is not the highest version tag found.

### ⛔️ Latest commit on the default branch lacks version tag

The latest commit on the default branch does not have a version tag.

### ✅ Modver analysis: no new version tag required

Modver is a tool that can compare two versions of a Go module
to determine whether the differences between them require a change in the major version number,
minor version number,
or patchlevel.

Taggo performs a Modver analysis between the latest version and the latest commit,
when the latest commit has no version tag.

This message means that the differences in the Go module, if any, do not require a new version.

### ⛔️ Modver analysis: ...

This message means that Modver found some differences requiring a new version tag.

### ⛔️ Recommended new version: ...

If Modver found differences requiring a new version,
this is the recommended new version tag
(including any required version prefix).

### ⛔️ Module path will require new version suffix ...

If Modver is recommending a new major version number,
and that number is 2 or higher,
the module path will need to be updated with a new version suffix to reflect that.
See [go.dev/ref/mod#major-version-suffixes](https://go.dev/ref/mod#major-version-suffixes).

Note: The module path is specified in `go.mod`,
but code changes in `.go` files may be needed too.
In particular,
if the module contains multiple packages,
and code in one of the module’s packages imports code from another,
you will need to update those `import` declarations
to reflect the new module path.

### ⛔️ No version tags

Taggo did not find any version tags for the module
(after removing any required version prefix).
Some findings will not be available as a result.

### ⛔️ Module path ... does not agree with module subdir in repository ...

The module root is in subdirectory `foo/bar` of its repository,
but the module path does not end with `/foo/bar`
(not counting any required major-version suffix).
See [go.dev/ref/mod#module-path](https://go.dev/ref/mod#module-path).

### ✅ Module path ... agrees with module subdir in repository ...

The module root is in a subdirectory of its repository,
and the module path includes that subdirectory.
