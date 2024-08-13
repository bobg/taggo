package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bobg/errors"

	"github.com/bobg/taggo"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)

		var (
			ee       exitErr
			exitCode = 1
		)
		if errors.As(err, &ee) {
			exitCode = ee.code
		}
		os.Exit(exitCode)
	}
}

func run() error {
	var (
		add    bool
		all    bool
		doJSON bool
		git    string
		msg    string
		quiet  bool
		sign   bool
		status bool
	)
	flag.BoolVar(&add, "add", false, "add any recommended new version tag to the repository")
	flag.BoolVar(&all, "all", false, "check all modules in the repository")
	flag.BoolVar(&doJSON, "json", false, "output in JSON format")
	flag.StringVar(&git, "git", "", "path to git binary")
	flag.StringVar(&msg, "m", "", "with -add, message for new version tag")
	flag.BoolVar(&quiet, "q", false, "quiet mode: print warnings only")
	flag.BoolVar(&sign, "s", false, "with -add, sign the new version tag")
	flag.BoolVar(&status, "status", false, "exit with status 2 if there are warnings")
	flag.Parse()

	var (
		repodir, moduledir string
		err                error
	)

	if git == "" {
		git, err = exec.LookPath("git")
		if err != nil {
			return errors.Wrap(err, "finding git binary")
		}
	}

	switch flag.NArg() {
	case 0:
		if all {
			repodir, err = searchUpwardFor(".", ".git")
			if err != nil {
				return errors.Wrap(err, "finding repository directory")
			}
		} else {
			repodir, moduledir, err = determineDirs(".")
			if err != nil {
				return errors.Wrap(err, "determining directories")
			}
		}
	case 1:
		if all {
			repodir, err = searchUpwardFor(flag.Arg(0), ".git")
			if err != nil {
				return errors.Wrapf(err, "finding repository directory from %s", flag.Arg(0))
			}
		} else {
			repodir, moduledir, err = determineDirs(flag.Arg(0))
			if err != nil {
				return errors.Wrapf(err, "determining directories from %s", flag.Arg(0))
			}
		}
	case 2:
		if all {
			return fmt.Errorf("cannot specify both -all and MODULEDIR")
		}
		repodir, moduledir = flag.Arg(0), flag.Arg(1)

	default:
		return fmt.Errorf("usage: %s [-add] [-all] [-git GIT] [-json] [-msg MSG] [-q] [-status] [REPODIR] [MODULEDIR]", os.Args[0])
	}

	ctx := context.Background()

	if add {
		// Taggo won't add tags to an unclean repo.
		if err = checkClean(ctx, git, repodir); err != nil {
			return errors.Wrap(err, "checking for clean repository")
		}
	}

	if all {
		modules, err := taggo.CheckAll(ctx, git, repodir)
		if err != nil {
			return errors.Wrapf(err, "checking all modules in %s", repodir)
		}

		if doJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			err := enc.Encode(modules)
			return errors.Wrap(err, "encoding result")
		}

		var (
			first    = true
			warnings int
			tagErrs  error
		)

		for mdir, result := range modules {
			if first {
				first = false
			} else {
				fmt.Println()
			}
			fmt.Printf("%s:\n\n", mdir)
			warnings += result.Describe(os.Stdout, quiet)

			if add {
				if err := maybeAddTag(ctx, git, repodir, result, sign, msg); err != nil {
					tagErrs = errors.Join(tagErrs, errors.Wrapf(err, "adding tag to module %s", mdir))
				}
			}
		}

		err = tagErrs

		if status && warnings > 0 {
			err = errors.Join(err, exitErr{code: 2, err: fmt.Errorf("warnings found")})
		}

		return err

	}

	result, err := taggo.Check(ctx, git, repodir, moduledir)
	if err != nil {
		return errors.Wrapf(err, "checking module %s in repository %s", moduledir, repodir)
	}

	if doJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		err := enc.Encode(result)
		return errors.Wrap(err, "encoding result")
	}

	warnings := result.Describe(os.Stdout, quiet)

	if add {
		err = maybeAddTag(ctx, git, repodir, result, sign, msg)
	}

	if status && warnings > 0 {
		err = errors.Join(err, exitErr{code: 2, err: fmt.Errorf("warnings found")})
	}

	return err
}

func determineDirs(dir string) (repodir, moduledir string, err error) {
	moduledir, err = searchUpwardFor(dir, "go.mod")
	if err != nil {
		return "", "", errors.Wrap(err, "finding module directory")
	}
	repodir, err = searchUpwardFor(dir, ".git")
	return repodir, moduledir, errors.Wrap(err, "finding repository directory")
}

func searchUpwardFor(dir, name string) (string, error) {
	for {
		path := filepath.Join(dir, name)
		_, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			if dir == "/" {
				return "", fmt.Errorf("no %s found", name)
			}
			dir, err = filepath.Abs(filepath.Join(dir, ".."))
			if err != nil {
				return "", errors.Wrap(err, "finding parent directory")
			}
			continue
		}
		if err != nil {
			return "", errors.Wrapf(err, "statting %s", path)
		}
		return dir, nil
	}
}

type exitErr struct {
	code int
	err  error
}

func (e exitErr) Error() string {
	return e.err.Error()
}

func (e exitErr) Unwrap() error {
	return e.err
}

// Code returns the exit code for this error.
// But if this error wraps another exitErr,
// then the result is the least common multiple of the two codes.
func (e exitErr) Code() int {
	var ee exitErr
	if errors.As(e.err, &ee) {
		return lcm(e.code, ee.Code())
	}
	return e.code
}

func lcm(a, b int) int {
	return a / gcd(a, b) * b
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func maybeAddTag(ctx context.Context, git, repodir string, r taggo.Result, sign bool, msg string) error {
	if r.DefaultBranch == "" {
		return nil
	}
	if r.LatestCommit == "" {
		return nil
	}
	if r.LatestCommitHasVersionTag {
		return nil
	}
	if r.NewMajor == 0 && r.NewMinor == 0 && r.NewPatch == 0 {
		return nil
	}

	bareTag := fmt.Sprintf("v%d.%d.%d", r.NewMajor, r.NewMinor, r.NewPatch)
	if bareTag == r.LatestVersion {
		return nil
	}
	tag := r.VersionPrefix + bareTag

	if r.NewMajor != r.LatestMajor {
		return exitErr{code: 3, err: fmt.Errorf("will not add new major-version tag %s", tag)}
	}

	if msg == "" {
		msg = fmt.Sprintf("Version %s added by Taggo", tag)
	}

	args := []string{"tag", "-m", msg}
	if sign {
		args = append(args, "-s")
	}
	args = append(args, tag, r.LatestCommit)

	cmd := exec.CommandContext(ctx, git, args...)
	cmd.Dir = repodir
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "running %s", cmd)
	}

	fmt.Printf("🪄 Added tag %s\n", tag)
	return nil
}

func checkClean(ctx context.Context, git, repodir string) error {
	cmd := exec.CommandContext(ctx, git, "status", "--porcelain")
	cmd.Dir = repodir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "creating stdout pipe")
	}
	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "starting %s", cmd)
	}
	defer cmd.Wait()

	var (
		clean = true
		sc    = bufio.NewScanner(stdout)
	)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "??") {
			continue
		}
		clean = false
		if _, err = io.Copy(io.Discard, stdout); err != nil {
			return errors.Wrap(err, "discarding output")
		}
		break
	}
	if err := sc.Err(); err != nil {
		return errors.Wrapf(err, "scanning output of %s", cmd)
	}

	if err := cmd.Wait(); err != nil {
		return errors.Wrapf(err, "waiting for %s", cmd)
	}

	if !clean {
		return fmt.Errorf("repository is not clean")
	}
	return nil
}
