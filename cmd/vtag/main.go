package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bobg/errors"

	"github.com/bobg/vtag"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		all    bool
		doJSON bool
		git    string
		quiet  bool
		status bool
	)
	flag.BoolVar(&all, "all", false, "check all modules in the repository")
	flag.BoolVar(&doJSON, "json", false, "output in JSON format")
	flag.StringVar(&git, "git", "", "path to git binary")
	flag.BoolVar(&quiet, "q", false, "quiet mode: print warnings only")
	flag.BoolVar(&status, "status", false, "exit with status 2 if there are warnings")
	flag.Parse()

	var (
		repodir, moduledir string
		err                error
	)

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
		return fmt.Errorf("usage: %s [-all] [-git GIT] [-json] [-q] [-status] [REPODIR] [MODULEDIR]", os.Args[0])
	}

	ctx := context.Background()

	if all {
		modules, err := vtag.CheckAll(ctx, git, repodir)
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
		)

		for mdir, result := range modules {
			if first {
				first = false
			} else {
				fmt.Println()
			}
			fmt.Printf("%s:\n\n", mdir)
			warnings += result.Describe(os.Stdout, quiet)
		}

		if status && warnings > 0 {
			os.Exit(2)
		}

		return nil

	}

	result, err := vtag.Check(ctx, git, repodir, moduledir)
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
	if status && warnings > 0 {
		os.Exit(2)
	}

	return nil
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
