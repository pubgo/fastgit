package ggccmd

import "testing"

func TestAliasDefRequiredArgCount(t *testing.T) {
	a := AliasDef{Single: "commit {0}"}
	if got, want := a.RequiredArgCount(), 1; got != want {
		t.Fatalf("RequiredArgCount()=%d want=%d", got, want)
	}

	b := AliasDef{Sequence: []string{"branch checkout {0}", "commit {1}"}}
	if got, want := b.RequiredArgCount(), 2; got != want {
		t.Fatalf("RequiredArgCount()=%d want=%d", got, want)
	}
}

func TestExpandAliasTemplate(t *testing.T) {
	got, err := expandAliasTemplate("commit {0}", []string{"fix: typo"})
	if err != nil {
		t.Fatalf("expandAliasTemplate error: %v", err)
	}
	if got != "commit fix: typo" {
		t.Fatalf("expanded=%q", got)
	}

	if _, err := expandAliasTemplate("commit {1}", []string{"only0"}); err == nil {
		t.Fatalf("expected missing argument error")
	}
}
