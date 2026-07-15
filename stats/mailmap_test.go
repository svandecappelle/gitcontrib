package stats

import "testing"

func TestMailmapResolve(t *testing.T) {
	content := `
# a comment
Proper Name <proper@e>
Renamed <canon@e> <old@e>
<canon2@e> <old2@e>
Bot Author <bot@e> semantic-release-bot <human@e>
`
	mm := parseMailmap(content)

	cases := []struct {
		name, email         string
		wantName, wantEmail string
	}{
		// Form 3: set name for that email, email kept.
		{"whatever", "proper@e", "Proper Name", "proper@e"},
		// Email match is case-insensitive.
		{"whatever", "PROPER@E", "Proper Name", "proper@e"},
		// Form 2: name + email replaced when commit email matches the old email.
		{"anything", "old@e", "Renamed", "canon@e"},
		// Form 1: only the email is remapped, the name is kept.
		{"Keep Me", "old2@e", "Keep Me", "canon2@e"},
		// Form 4: replaced only when BOTH name and email match.
		{"semantic-release-bot", "human@e", "Bot Author", "bot@e"},
		// Same email but a different name: form 4 does not apply, unchanged.
		{"Real Human", "human@e", "Real Human", "human@e"},
		// Unknown identity: unchanged.
		{"Nobody", "none@e", "Nobody", "none@e"},
	}
	for _, c := range cases {
		gotName, gotEmail := mm.Resolve(c.name, c.email)
		if gotName != c.wantName || gotEmail != c.wantEmail {
			t.Errorf("Resolve(%q,%q) = (%q,%q), want (%q,%q)",
				c.name, c.email, gotName, gotEmail, c.wantName, c.wantEmail)
		}
	}
}

func TestMailmapNilResolve(t *testing.T) {
	var mm *Mailmap
	name, email := mm.Resolve("Alice", "alice@e")
	if name != "Alice" || email != "alice@e" {
		t.Errorf("nil Mailmap Resolve changed the identity: (%q,%q)", name, email)
	}
}
