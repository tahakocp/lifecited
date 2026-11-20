package quote

import "testing"

func TestContains(t *testing.T) {
	page := "Wearing a seat-belt can reduce the risk of death among vehicle occupants by up to 50%.\n" +
		"Drop and coveruntil shaking stops. It’s safe — mostly."
	ix := NewIndex(page)
	cases := []struct {
		q    string
		want bool
	}{
		{"Wearing a seat-belt can reduce the risk of death among vehicle occupants by up to 50%.", true},
		{"wearing a seat belt can reduce the risk", true},        // case, hyphen
		{"Drop and cover until shaking stops", true},             // words glued by stripped inline tags
		{"It's safe - mostly", true},                             // curly apostrophe, dash
		{"Wearing a seat-belt ... up to 50%", true},              // ellipsis
		{"Wearing a seat-belt … up to 50%", true},                // unicode ellipsis
		{"up to 50% ... Wearing a seat-belt", false},             // order matters
		{"by up to 60%", false},                                  // changed number
		{"reduce the risk of death among all road users", false}, // changed words
	}
	for _, c := range cases {
		if got := ix.Contains(c.q); got != c.want {
			t.Errorf("Contains(%q) = %v, want %v", c.q, got, c.want)
		}
	}
}

func TestClosest(t *testing.T) {
	ix := NewIndex("The risk rises 4.5 times from 50 km/h to 65 km/h.")
	m, n := ix.Closest("The risk rises 4.5 times from 50 km/h to 70 km/h.")
	if n != len(Fold("The risk rises 4.5 times from 50 km/h to 70 km/h.")) {
		t.Fatalf("total %d", n)
	}
	if m != len(Fold("The risk rises 4.5 times from 50 km/h to ")) {
		t.Errorf("matched %d of %d", m, n)
	}
}
