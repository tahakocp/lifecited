package book

import (
	"errors"
	"strings"
	"testing"
)

const good = `# 1. Test chapter

Intro line.

### 1. Wear a seat belt
- Cost: nothing.
- Gain: cuts the risk of death by up to 50%.
- Grade: B
- Source: [WHO, Road traffic injuries](https://www.who.int/x)
  > Wearing a seat-belt can reduce the risk of death among vehicle occupants by up to 50%.
- Note: back seats count.

### 2. Second
- Cost: a fee of 10,000 over time.
- Gain: saves 10,000 and 4.5 times the risk.
- Grade: A
- Source: [A](https://example.org/a)
  > it costs 10000 in total
- Source: [B](https://example.org/b_(x))
  > raises it 4.5 times
`

func TestParse(t *testing.T) {
	ch, err := Parse("01-test.md", strings.NewReader(good))
	if err != nil {
		t.Fatal(err)
	}
	if ch.Number != 1 || ch.Title != "Test chapter" || len(ch.Intro) != 1 {
		t.Fatalf("chapter header: %+v", ch)
	}
	if len(ch.Entries) != 2 {
		t.Fatalf("entries: %d", len(ch.Entries))
	}
	e := ch.Entries[0]
	if e.ID() != "1.1" || e.Grade != "B" || len(e.Sources) != 1 || len(e.Sources[0].Quotes) != 1 || len(e.Notes) != 1 {
		t.Errorf("entry 1: %+v", e)
	}
	if got := ch.Entries[1].Sources[1].URL; got != "https://example.org/b_(x)" {
		t.Errorf("url with parentheses: %q", got)
	}
	if probs := Lint([]*Chapter{ch}); len(probs) != 0 {
		t.Errorf("lint: %v", probs)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"no header":        "### 1. x\n",
		"bad numbering":    "# 1. C\n### 2. x\n",
		"stray text":       "# 1. C\n### 1. x\nsome prose\n",
		"orphan quote":     "# 1. C\n### 1. x\n  > quote\n",
		"bare url":         "# 1. C\n### 1. x\n- Source: https://example.org\n",
		"http not allowed": "# 1. C\n### 1. x\n- Source: [a](http://example.org)\n",
	}
	for name, in := range cases {
		_, err := Parse("t.md", strings.NewReader(in))
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Errorf("%s: want ParseError, got %v", name, err)
		}
	}
}

func TestLint(t *testing.T) {
	in := `# 1. C

### 1. Made-up number
- Cost: x
- Gain: cuts deaths by 60%.
- Grade: B
- Source: [s](https://example.org)
  > cuts deaths by up to 50%.

### 2. Missing things
- Gain: something
- Grade: D
- Source: [s](https://example.org)
`
	ch, err := Parse("t.md", strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	var msgs []string
	for _, p := range Lint([]*Chapter{ch}) {
		msgs = append(msgs, p.Entry.ID()+" "+p.Msg)
	}
	want := []string{
		"1.1 number 60 in Gain is not in any quote",
		"1.2 missing Cost",
		`1.2 Grade must be A, B or C, got "D"`,
		"1.2 source https://example.org has no quote",
	}
	if strings.Join(msgs, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(msgs, "\n"), strings.Join(want, "\n"))
	}
}

// The real guide must always pass lint.
func TestRealBook(t *testing.T) {
	chs, err := Load("../../book")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range Lint(chs) {
		t.Error(p)
	}
	n, g, _, _ := Count(chs)
	if n < 50 || g["A"]+g["B"]+g["C"] != n {
		t.Errorf("entries %d grades %v", n, g)
	}
}
