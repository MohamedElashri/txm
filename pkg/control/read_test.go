package control

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
)

func TestDefaultReadPolicyCannotBeBroadenedByTargetArgument(t *testing.T) {
	r, err := NewReader(core.ReadScope{Backends: []string{"native"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Agent(context.Background(), strings.Repeat("a", 32)); err == nil {
		t.Fatal("target argument granted itself read permission")
	}
	id := strings.Repeat("a", 32)
	scope := core.ReadScope{Backends: []string{"native"}, ManagedIDs: []string{id}}
	r, err = NewReader(scope)
	if err != nil {
		t.Fatal(err)
	}
	scope.ManagedIDs[0] = strings.Repeat("b", 32)
	if _, err = r.scoped(context.Background(), strings.Repeat("b", 32)); err == nil {
		t.Fatal("caller mutated fixed policy")
	}
}

func TestReadRepositoryReplacementInvalidatesStartupPolicy(t *testing.T) {
	if !runtimepath.Supported() {
		t.Skip("Unix filesystem identity")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(base, "repo")
	if err = os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	r, err := NewReader(core.ReadScope{Backends: []string{"native"}, Repositories: []string{repo}})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(repo, repo+"-old"); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Agents(context.Background()); err == nil {
		t.Fatal("replacement repository inherited read grant")
	}
}
