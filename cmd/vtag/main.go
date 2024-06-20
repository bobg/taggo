package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

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

	var repodir, moduledir string
	repodir = "."

	if flag.NArg() > 0 {
		repodir = flag.Arg(0)
	}
	if flag.NArg() > 1 {
		moduledir = flag.Arg(1)
	}
	if flag.NArg() > 2 {
		return fmt.Errorf("unexpected extra arguments")
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
