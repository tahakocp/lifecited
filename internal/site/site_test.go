package site

import (
	"strings"
	"testing"
	"time"

	"github.com/TahaKocpinar/lifecited/internal/book"
	"github.com/TahaKocpinar/lifecited/internal/check"
)

func TestRender(t *testing.T) {
	md := "# 2. First aid\n\nIntro <b>text</b>.\n\n### 1. Burns\n- Cost: 20 minutes\n- Gain: cool <it>\n- Grade: C\n- Source: [NHS](https://www.nhs.uk/x)\n  > Hold under cool water\n"
	ch, err := book.Parse("02-first-aid.md", strings.NewReader(md))
	if err != nil {
		t.Fatal(err)
	}
	ch.Slug = "first-aid"
	rep := &check.Report{Started: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Counts: map[check.Status]int{check.OK: 1},
		Results: []check.QuoteResult{{EntryID: "2.1", URL: "https://www.nhs.uk/x", Quote: "Hold under cool water", Status: check.OK}}}
	out, err := Render([]*book.Chapter{ch}, Options{Report: rep, Repo: "https://github.com/x/y", Now: rep.Started})
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)
	for _, want := range []string{`id="e2-1"`, `data-g="C"`, "quote found", "1 of 1 quotes found", "Intro &lt;b&gt;text&lt;/b&gt;.", "cool &lt;it&gt;", `href="https://www.nhs.uk/x"`} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(html, "<it>") || strings.Contains(html, "src=\"http") {
		t.Error("unescaped input or external script")
	}
}
