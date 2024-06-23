package taggo

import (
	"bufio"
	"context"
	"os/exec"
	"strings"

	"github.com/bobg/errors"
)

func gitRefs(ctx context.Context, git, dir string, f func(name, hash string) error) error {
	cmd := exec.CommandContext(ctx, git, "show-ref")
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "creating stdout pipe")
	}
	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "starting %s", cmd)
	}
	defer cmd.Wait()

	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue // silently ignore malformed lines
		}
		hash, name := fields[0], fields[1]
		if err := f(name, hash); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		return errors.Wrapf(err, "scanning output of %s", cmd)
	}
	err = cmd.Wait()
	return errors.Wrapf(err, "waiting for %s", cmd)
}
