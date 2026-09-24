package cli

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bryanbarton525/prism/internal/extensions"
	"github.com/bryanbarton525/prism/internal/filelock"
	"github.com/spf13/cobra"
)

func TestManagedAgentAndSkillCommandsPropagateCancellation(t *testing.T) {
	orig := gf.stateDir
	gf.stateDir = t.TempDir()
	t.Cleanup(func() { gf.stateDir = orig })
	unlock, err := filelock.Acquire(context.Background(), extensions.NewStore(gf.stateDir).LockPath(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	for _, cmd := range []*cobra.Command{newAgentManagedRemoveCmd(), newSkillManagedRemoveCmd()} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		cmd.SetArgs([]string{"missing"})
		if err := cmd.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("%s ignored command cancellation: %v", cmd.Name(), err)
		}
	}
}
