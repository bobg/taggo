package taggo_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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
				result map[string]taggo.Result
				dec    = json.NewDecoder(f)
			)
			if err := dec.Decode(&result); err != nil {
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

			got, err := taggo.CheckAll(context.Background(), "", tmpdir)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(result, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
