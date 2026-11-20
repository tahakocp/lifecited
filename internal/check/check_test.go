package check

import (
	"errors"
	"strings"
	"testing"

	"github.com/tahakocp/lifecited/internal/book"
	"github.com/tahakocp/lifecited/internal/source"
)

type fakeFetcher map[string]*source.Page

func (f fakeFetcher) Fetch(u string) (*source.Page, error) {
	if p, ok := f[u]; ok {
		return p, nil
	}
	return nil, errors.New("HTTP 403")
}

func TestRun(t *testing.T) {
	md := `# 1. C

### 1. A
- Cost: x
- Gain: y
- Grade: C
- Source: [a](https://a.example/page)
  > Hold the burn under cool running water for 20 minutes.
  > Hold it for 30 minutes.
- Source: [b](https://b.example/x)
  > anything

### 2. B
- Cost: x
- Gain: y
- Grade: C
- Source: [a](https://a.example/page)
  > cool running water
- Source: [c](https://c.example/old)
  > moved text
`
	ch, err := book.Parse("01-c.md", strings.NewReader(md))
	if err != nil {
		t.Fatal(err)
	}
	f := fakeFetcher{
		"https://a.example/page": {Text: "Hold the burn under cool running water for 20 minutes."},
		"https://c.example/old":  {Text: "Some moved text here", FinalURL: "https://c.example/new"},
	}
	f["https://a.example/page"].FinalURL = "https://a.example/page/"
	rep := Run([]*book.Chapter{ch}, f, 2)
	if rep.Pages != 3 {
		t.Errorf("pages %d", rep.Pages)
	}
	var got []string
	for _, r := range rep.Results {
		got = append(got, r.EntryID+" "+string(r.Status)+" "+r.Moved)
	}
	want := []string{"1.1 ok ", "1.1 missing ", "1.1 unreachable ", "1.2 ok ", "1.2 ok https://c.example/new"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got  %v\nwant %v", got, want)
	}
	if rep.Counts[OK] != 3 || rep.Counts[Missing] != 1 || rep.Counts[Unreachable] != 1 {
		t.Errorf("counts %v", rep.Counts)
	}
	if r := rep.Results[1]; r.Matched == 0 || r.Matched >= r.Total {
		t.Errorf("closest %d/%d", r.Matched, r.Total)
	}
}
