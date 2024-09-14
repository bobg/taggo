package taggo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/bobg/go-generics/v3/maps"
	"github.com/google/go-cmp/cmp"

	"github.com/bobg/taggo"
)

func TestCheckAll(t *testing.T) {
	var (
		doUpdate  = os.Getenv("UPDATE_GOLDEN") == "true"
		didUpdate bool
	)
	if doUpdate {
		t.Log("Will update golden files")
		defer func() {
			if didUpdate {
				t.Error("updated golden files")
			} else {
				t.Log("No golden-file updates needed")
			}
		}()
	}

	tests, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		if !test.IsDir() {
			continue
		}

		var (
			testName = test.Name()
			testPath = filepath.Join("testdata", testName)
		)

		t.Run(testName, func(t *testing.T) {
			resultPath := filepath.Join(testPath, "result.json")
			f, err := os.Open(resultPath)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			var (
				want []taggo.Result
				dec  = json.NewDecoder(f)
			)
			if err := dec.Decode(&want); err != nil {
				t.Fatal(err)
			}

			tmpdir, err := os.MkdirTemp("", "taggo")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmpdir)

			bundlePath := filepath.Join(testPath, "bundle")
			cmd := exec.Command("git", "clone", "-c", "init.defaultBranch=main", bundlePath, tmpdir)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				t.Fatal(err)
			}

			gotMap, err := taggo.CheckAll(context.Background(), "", tmpdir)
			if err != nil {
				t.Fatal(err)
			}

			got := maps.Values(gotMap)
			sort.Slice(got, func(i, j int) bool { return got[i].ModuleSubdir < got[j].ModuleSubdir })

			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}

			for _, result := range got {
				goldenFile := filepath.Join(testPath, "golden")
				if moduleSubdir := strings.ReplaceAll(result.ModuleSubdir, "/", "_"); moduleSubdir != "" {
					goldenFile += "-" + moduleSubdir
				}

				var desc bytes.Buffer
				result.Describe(&desc, false)

				want, err := os.ReadFile(goldenFile)
				if err != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(string(want), desc.String()); diff != "" {
					if doUpdate {
						if err := os.WriteFile(goldenFile, desc.Bytes(), 0644); err != nil {
							t.Fatal(err)
						}
						didUpdate = true
						continue
					}
					t.Errorf("mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
