package taggo_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/bobg/go-generics/v3/maps"
	"github.com/google/go-cmp/cmp"

	"github.com/bobg/taggo"
)

func TestCheckAll(t *testing.T) {
	tests, err := os.ReadDir("_testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		if !test.IsDir() {
			continue
		}

		var (
			testName = test.Name()
			testPath = filepath.Join("_testdata", testName)
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
			cmd := exec.Command("git", "clone", bundlePath, tmpdir)
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
		})
	}
}
