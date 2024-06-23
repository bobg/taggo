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
		doJSON bool
		git    string
		quiet  bool
	)
	flag.BoolVar(&doJSON, "json", false, "output in JSON format")
	flag.StringVar(&git, "git", "", "path to git binary")
	flag.BoolVar(&quiet, "q", false, "quiet mode: print warnings only")
	flag.Parse()

	var (
		repodir, moduledir string
		err                error
	)

	switch flag.NArg() {
	case 0:
		repodir, moduledir, err = determineDirs(".")
	case 1:
		repodir, moduledir, err = determineDirs(flag.Arg(0))
	case 2:
		repodir, moduledir = flag.Arg(0), flag.Arg(1)
	default:
		return fmt.Errorf("Usage: %s [-json] [-git PATH] [-q] [REPODIR [MODULEDIR]]", os.Args[0])
	}

	ctx := context.Background()

	result, err := vtag.Check(ctx, git, repodir, moduledir)
	if err != nil {
		return err
	}

	if doJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		err := enc.Encode(result)
		return errors.Wrap(err, "encoding result")
	}

	result.Describe(os.Stdout, quiet)

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
