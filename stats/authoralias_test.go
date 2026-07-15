package stats

import (
	"reflect"
	"sort"
	"testing"
)

func edKey(name, email string) string { return name + authorIDSep + email }

func TestSplitAuthorKey(t *testing.T) {
	cases := []struct{ key, name, email string }{
		{"Alice" + authorIDSep + "a@e", "Alice", "a@e"},
		{"Alice" + authorIDSep, "Alice", ""},
		{"noseparator", "noseparator", ""},
	}
	for _, c := range cases {
		n, e := splitAuthorKey(c.key)
		if n != c.name || e != c.email {
			t.Errorf("splitAuthorKey(%q) = (%q,%q), want (%q,%q)", c.key, n, e, c.name, c.email)
		}
	}
}

func TestNicerName(t *testing.T) {
	if !nicerName("Romain Guisset", "romain.guisset") {
		t.Error("a name with a space should be nicer than one without")
	}
	if nicerName("romain.guisset", "Romain Guisset") {
		t.Error("a name without a space should not be nicer than one with")
	}
	if !nicerName("anything", "") {
		t.Error("any name should be nicer than empty")
	}
}

func TestMergeAuthorAliasesSameEmailHigherVolumeWins(t *testing.T) {
	got := mergeAuthorAliases(map[string][2]int{
		edKey("romain.guisset", "r@e"): {10, 0},
		edKey("Romain Guisset", "r@e"): {5, 0},
	})
	if len(got) != 1 {
		t.Fatalf("want 1 contributor, got %d: %+v", len(got), got)
	}
	if got[0].Author != "romain.guisset" || got[0].Total != 15 {
		t.Errorf("got Author=%q Total=%d, want romain.guisset/15", got[0].Author, got[0].Total)
	}
	if !reflect.DeepEqual(got[0].Identities, []string{"r@e"}) {
		t.Errorf("Identities = %v, want [r@e]", got[0].Identities)
	}
}

func TestMergeAuthorAliasesTiePrefersProperName(t *testing.T) {
	got := mergeAuthorAliases(map[string][2]int{
		edKey("romain.guisset", "r@e"): {5, 0},
		edKey("Romain Guisset", "r@e"): {5, 0},
	})
	if len(got) != 1 || got[0].Author != "Romain Guisset" {
		t.Errorf("want single contributor 'Romain Guisset', got %+v", got)
	}
}

func TestMergeAuthorAliasesSameNameTwoEmailsStaySeparate(t *testing.T) {
	got := mergeAuthorAliases(map[string][2]int{
		edKey("Alice", "a@x"): {1, 0},
		edKey("Alice", "a@y"): {1, 0},
	})
	if len(got) != 2 {
		t.Fatalf("want 2 contributors (one per email), got %d: %+v", len(got), got)
	}
}

func TestMergeAuthorAliasesNoBridgeThroughBot(t *testing.T) {
	// A bot authoring under two humans' emails must not bridge them.
	got := mergeAuthorAliases(map[string][2]int{
		edKey("Alice", "a@x"): {1, 0},
		edKey("bot", "a@x"):   {1, 0},
		edKey("Bob", "b@y"):   {1, 0},
		edKey("bot", "b@y"):   {1, 0},
	})
	if len(got) != 2 {
		t.Fatalf("want 2 contributors (one per email, no bridge), got %d: %+v", len(got), got)
	}
	for _, c := range got {
		ids := append([]string{}, c.Identities...)
		sort.Strings(ids)
		if reflect.DeepEqual(ids, []string{"a@x", "b@y"}) {
			t.Errorf("a contributor bridged both emails: %+v", c)
		}
	}
}

func TestMergeAuthorAliasesEmailless(t *testing.T) {
	got := mergeAuthorAliases(map[string][2]int{
		edKey("Solo", ""): {3, 0},
	})
	if len(got) != 1 || got[0].Author != "Solo" || got[0].Total != 3 {
		t.Fatalf("want Solo/3, got %+v", got)
	}
	if !reflect.DeepEqual(got[0].Identities, []string{"Solo"}) {
		t.Errorf("Identities = %v, want [Solo]", got[0].Identities)
	}
}

func TestMergeAuthorAliasesSortedByTotal(t *testing.T) {
	got := mergeAuthorAliases(map[string][2]int{
		edKey("Small", "s@e"): {1, 0},
		edKey("Big", "b@e"):   {10, 0},
		edKey("Mid", "m@e"):   {5, 0},
	})
	if len(got) != 3 {
		t.Fatalf("want 3, got %d", len(got))
	}
	if !(got[0].Total >= got[1].Total && got[1].Total >= got[2].Total) {
		t.Errorf("contributors not sorted by total desc: %+v", got)
	}
	if got[0].Author != "Big" {
		t.Errorf("top contributor = %q, want Big", got[0].Author)
	}
}
