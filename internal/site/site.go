// Package site renders the guide as one self-contained HTML page: no
// scripts or fonts from other hosts, so the file also works saved to a phone.
package site

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/TahaKocpinar/lifecited/internal/book"
	"github.com/TahaKocpinar/lifecited/internal/check"
)

//go:embed page.html
var pageHTML string

type quoteView struct {
	Text   string
	Status string // ok, missing, unreachable, or "" if no report
}

type sourceView struct {
	Label, URL string
	Quotes     []quoteView
}

type entryView struct {
	ID, Anchor, Title, Cost, Gain, Grade string
	Notes                                []string
	Sources                              []sourceView
	Search                               string
}

type chapterView struct {
	Number  int
	Title   string
	Anchor  string
	Intro   []string
	Entries []entryView
}

type data struct {
	Chapters  []chapterView
	Entries   int
	Grades    map[string]int
	Pages     int
	Quotes    int
	CheckedOn string
	QuotesOK  int
	Repo      string
	BuiltOn   string
}

// Options for Render.
type Options struct {
	Report *check.Report // optional: show per-quote check status
	Repo   string        // link in the footer
	Now    time.Time
}

func Render(chs []*book.Chapter, opt Options) ([]byte, error) {
	status := map[string]string{} // entryID|url|quote -> status
	d := data{Repo: opt.Repo, BuiltOn: opt.Now.UTC().Format("2006-01-02")}
	if opt.Report != nil {
		d.CheckedOn = opt.Report.Started.Format("2006-01-02")
		d.QuotesOK = opt.Report.Counts[check.OK]
		for _, r := range opt.Report.Results {
			status[r.EntryID+"|"+r.URL+"|"+r.Quote] = string(r.Status)
		}
	}
	d.Entries, d.Grades, d.Pages, d.Quotes = book.Count(chs)
	for _, ch := range chs {
		cv := chapterView{Number: ch.Number, Title: ch.Title, Anchor: ch.Slug, Intro: ch.Intro}
		for _, e := range ch.Entries {
			ev := entryView{ID: e.ID(), Anchor: "e" + strings.ReplaceAll(e.ID(), ".", "-"), Title: e.Title,
				Cost: e.Cost, Gain: e.Gain, Grade: e.Grade, Notes: e.Notes}
			words := []string{e.Title, e.Cost, e.Gain, strings.Join(e.Notes, " "), ch.Title}
			for _, s := range e.Sources {
				sv := sourceView{Label: s.Label, URL: s.URL}
				for _, q := range s.Quotes {
					sv.Quotes = append(sv.Quotes, quoteView{q, status[e.ID()+"|"+s.URL+"|"+q]})
				}
				ev.Sources = append(ev.Sources, sv)
				words = append(words, s.Label)
			}
			ev.Search = strings.ToLower(strings.Join(words, " "))
			cv.Entries = append(cv.Entries, ev)
		}
		d.Chapters = append(d.Chapters, cv)
	}
	t, err := template.New("page").Funcs(template.FuncMap{
		"pct": func(a, b int) string {
			if b == 0 {
				return "0"
			}
			return fmt.Sprintf("%d", a*100/b)
		},
	}).Parse(pageHTML)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
