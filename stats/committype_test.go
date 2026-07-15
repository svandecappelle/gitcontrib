package stats

import "testing"

func TestCommitType(t *testing.T) {
	cases := []struct {
		message string
		want    string
	}{
		{"feat: add thing", "feat"},
		{"fix(scope): bug", "fix"},
		{"feat!: breaking", "feat"},
		{"refactor(a/b)!: rework", "refactor"},
		{"docs:no space", "docs"},
		{"FEAT: upper", "feat"},            // lowercased
		{"chore: x\n\nbody line", "chore"}, // only the first line matters
		{"Merge branch 'main'", "other"},
		{"random message", "other"},
		{"wip: not a known type", "other"},
		{"", "other"},
		{"123: leading digits", "other"}, // type must be letters
	}
	for _, c := range cases {
		if got := commitType(c.message); got != c.want {
			t.Errorf("commitType(%q) = %q, want %q", c.message, got, c.want)
		}
	}
}
