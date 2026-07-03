package planner

import (
	"testing"

	"github.com/go-git/go-git/v6/plumbing"
)

func TestIsRefExcluded(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		ref      string
		prefixes []string
		exact    []string
		want     bool
	}{
		{"no filters", "refs/heads/main", nil, nil, false},

		// Prefix matching (unchanged behavior).
		{"prefix under", "refs/pull/1/head", []string{"refs/pull/"}, nil, true},
		{"prefix boundary sibling", "refs/pullx", []string{"refs/pull/"}, nil, false},
		{"prefix blank entry skipped", "refs/heads/main", []string{"  "}, nil, false},

		// Exact matching: matches the whole name only, never children — so a
		// caller can reserve refs/heads/entire while still mirroring
		// refs/heads/entire/foo.
		{"exact hit", "refs/heads/entire", nil, []string{"refs/heads/entire"}, true},
		{"exact does not match child", "refs/heads/entire/foo", nil, []string{"refs/heads/entire"}, false},
		{"exact does not match sibling", "refs/heads/entirely", nil, []string{"refs/heads/entire"}, false},
		{"exact blank entry skipped", "refs/heads/main", nil, []string{" "}, false},

		// Combined: prefix and exact together.
		{"prefix wins", "refs/heads/entire/unmirrored/x", []string{"refs/heads/entire/unmirrored/"}, []string{"refs/heads/entire"}, true},
		{"exact wins", "refs/heads/entire", []string{"refs/heads/entire/unmirrored/"}, []string{"refs/heads/entire"}, true},
		{"neither", "refs/heads/entire/foo", []string{"refs/heads/entire/unmirrored/"}, []string{"refs/heads/entire"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := IsRefExcluded(plumbing.ReferenceName(c.ref), c.prefixes, c.exact)
			if got != c.want {
				t.Errorf("IsRefExcluded(%q, %v, %v) = %v, want %v", c.ref, c.prefixes, c.exact, got, c.want)
			}
		})
	}
}
