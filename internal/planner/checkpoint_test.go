package planner

import (
	"strings"
	"testing"

	git "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/storage/memory"
)

// When a first-parent is absent from the store, the error path must not
// dereference the (nil) commit it just failed to load, and must name the
// parent hash it could not find.
func TestFirstParentChainStoppingAtMissingParentErrors(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), nil)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}
	missingParent := plumbing.NewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	tip := seedCommit(t, repo, []plumbing.Hash{missingParent})

	_, err = FirstParentChainStoppingAt(repo.Storer, tip, map[plumbing.Hash]struct{}{})
	if err == nil {
		t.Fatal("expected error when first parent is missing from the store")
	}
	if !strings.Contains(err.Error(), missingParent.String()) {
		t.Fatalf("error should name the missing parent %s, got %q", missingParent, err.Error())
	}
}
