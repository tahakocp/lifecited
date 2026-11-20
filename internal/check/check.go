// Package check fetches every cited page once and confirms each quote is
// still on it.
package check

import (
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/TahaKocpinar/lifecited/internal/book"
	"github.com/TahaKocpinar/lifecited/internal/quote"
	"github.com/TahaKocpinar/lifecited/internal/source"
)

type Status string

const (
	OK          Status = "ok"
	Missing     Status = "missing"     // page read, quote not on it
	Unreachable Status = "unreachable" // page could not be read
)

type QuoteResult struct {
	Entry   *book.Entry  `json:"-"`
	Source  *book.Source `json:"-"`
	EntryID string       `json:"entry"`
	URL     string       `json:"url"`
	Quote   string       `json:"quote"`
	Status  Status       `json:"status"`
	Matched int          `json:"matched,omitempty"` // folded chars found, for missing quotes
	Total   int          `json:"total,omitempty"`
	Err     string       `json:"error,omitempty"`
	Moved   string       `json:"moved_to,omitempty"` // redirect to another host
	Checked time.Time    `json:"checked"`
}

type Report struct {
	Started time.Time      `json:"started"`
	Results []QuoteResult  `json:"results"`
	Pages   int            `json:"pages"`
	Counts  map[Status]int `json:"counts"`
}

type Fetcher interface {
	Fetch(string) (*source.Page, error)
}

// Run checks all quotes. Each URL is fetched once, with at most `workers`
// requests in flight.
func Run(chs []*book.Chapter, f Fetcher, workers int) *Report {
	rep := &Report{Started: time.Now().UTC(), Counts: map[Status]int{}}
	byURL := map[string][]QuoteResult{}
	var order []string
	for _, ch := range chs {
		for _, e := range ch.Entries {
			for _, s := range e.Sources {
				if _, ok := byURL[s.URL]; !ok {
					order = append(order, s.URL)
				}
				for _, q := range s.Quotes {
					byURL[s.URL] = append(byURL[s.URL], QuoteResult{Entry: e, Source: s, EntryID: e.ID(), URL: s.URL, Quote: q})
				}
			}
		}
	}
	type page struct {
		p   *source.Page
		err error
	}
	pages := make(map[string]page, len(order))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, max(workers, 1))
	for _, u := range order {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			sem <- struct{}{}
			p, err := f.Fetch(u)
			<-sem
			mu.Lock()
			pages[u] = page{p, err}
			mu.Unlock()
		}(u)
	}
	wg.Wait()
	rep.Pages = len(order)
	for _, u := range order {
		pg := pages[u]
		var ix quote.Index
		if pg.err == nil {
			ix = quote.NewIndex(pg.p.Text)
		}
		for _, r := range byURL[u] {
			r.Checked = rep.Started
			switch {
			case pg.err != nil:
				r.Status, r.Err = Unreachable, pg.err.Error()
			case ix.Contains(r.Quote):
				r.Status = OK
			default:
				r.Status = Missing
				r.Matched, r.Total = ix.Closest(r.Quote)
			}
			if pg.err == nil && moved(u, pg.p.FinalURL) {
				r.Moved = pg.p.FinalURL
			}
			rep.Counts[r.Status]++
			rep.Results = append(rep.Results, r)
		}
	}
	sort.SliceStable(rep.Results, func(i, j int) bool {
		a, b := rep.Results[i].Entry, rep.Results[j].Entry
		if a.Chapter.Number != b.Chapter.Number {
			return a.Chapter.Number < b.Chapter.Number
		}
		return a.Number < b.Number
	})
	return rep
}

// moved reports a redirect to a different page. A trailing slash or a
// fragment does not count.
func moved(from, to string) bool {
	a, err1 := url.Parse(from)
	b, err2 := url.Parse(to)
	if err1 != nil || err2 != nil {
		return from != to
	}
	return a.Host != b.Host || strings.TrimSuffix(a.Path, "/") != strings.TrimSuffix(b.Path, "/") || a.RawQuery != b.RawQuery
}
