// Package book parses the guide's Markdown chapters into entries and checks
// that every entry has the fields the guide promises.
package book

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Chapter struct {
	File    string // e.g. 01-dont-die-early.md
	Slug    string // e.g. dont-die-early
	Number  int
	Title   string
	Intro   []string
	Entries []*Entry
}

type Entry struct {
	Chapter *Chapter
	Number  int
	Title   string
	Line    int
	Cost    string
	Gain    string
	Grade   string
	Notes   []string
	Sources []*Source
}

type Source struct {
	Label  string
	URL    string
	Quotes []string
	Line   int
}

// ID is a stable anchor such as "1.4".
func (e *Entry) ID() string { return fmt.Sprintf("%d.%d", e.Chapter.Number, e.Number) }

var (
	chapterFile = regexp.MustCompile(`^(\d\d)-([a-z0-9-]+)\.md$`)
	chapterHead = regexp.MustCompile(`^# (\d+)\. (.+)$`)
	entryHead   = regexp.MustCompile(`^### (\d+)\. (.+)$`)
	field       = regexp.MustCompile(`^- (Cost|Gain|Grade|Source|Note): (.*)$`)
	sourceLink  = regexp.MustCompile(`^\[(.+)\]\((https://[^\s)]+(?:\([^\s)]*\))?[^\s)]*)\)$`)
	quoteLine   = regexp.MustCompile(`^  > (.+)$`)
)

// Load reads every NN-slug.md file in dir, in order.
func Load(dir string) ([]*Chapter, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	var chs []*Chapter
	for _, path := range matches {
		m := chapterFile.FindStringSubmatch(filepath.Base(path))
		if m == nil {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		ch, err := Parse(filepath.Base(path), f)
		f.Close()
		if err != nil {
			return nil, err
		}
		ch.Slug = m[2]
		chs = append(chs, ch)
	}
	if len(chs) == 0 {
		return nil, fmt.Errorf("no chapters (NN-name.md) in %s", dir)
	}
	return chs, nil
}

// ParseError points at the line that broke the format.
type ParseError struct {
	File string
	Line int
	Msg  string
}

func (e *ParseError) Error() string { return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Msg) }

// Parse reads one chapter. It is strict on purpose: a line the parser does
// not understand inside an entry is an error, so nothing slips past the checks.
func Parse(name string, r io.Reader) (*Chapter, error) {
	ch := &Chapter{File: name}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var cur *Entry
	var src *Source
	n := 0
	fail := func(format string, a ...any) error {
		return &ParseError{name, n, fmt.Sprintf(format, a...)}
	}
	for sc.Scan() {
		n++
		line := strings.TrimRight(sc.Text(), " \t")
		switch {
		case n == 1:
			m := chapterHead.FindStringSubmatch(line)
			if m == nil {
				return nil, fail(`first line must be "# N. Title"`)
			}
			fmt.Sscan(m[1], &ch.Number)
			ch.Title = m[2]
		case entryHead.MatchString(line):
			m := entryHead.FindStringSubmatch(line)
			cur = &Entry{Chapter: ch, Title: m[2], Line: n}
			fmt.Sscan(m[1], &cur.Number)
			if want := len(ch.Entries) + 1; cur.Number != want {
				return nil, fail("entry number %d, expected %d", cur.Number, want)
			}
			ch.Entries = append(ch.Entries, cur)
			src = nil
		case cur == nil:
			if line != "" {
				ch.Intro = append(ch.Intro, line)
			}
		case line == "":
		case quoteLine.MatchString(line):
			if src == nil {
				return nil, fail("quote without a Source line above it")
			}
			src.Quotes = append(src.Quotes, quoteLine.FindStringSubmatch(line)[1])
		default:
			m := field.FindStringSubmatch(line)
			if m == nil {
				return nil, fail("unexpected line in entry %d: %q", cur.Number, line)
			}
			key, val := m[1], m[2]
			src = nil
			switch key {
			case "Cost":
				cur.Cost = val
			case "Gain":
				cur.Gain = val
			case "Grade":
				cur.Grade = val
			case "Note":
				cur.Notes = append(cur.Notes, val)
			case "Source":
				l := sourceLink.FindStringSubmatch(val)
				if l == nil {
					return nil, fail("Source must be a single [label](https://url)")
				}
				src = &Source{Label: l[1], URL: l[2], Line: n}
				cur.Sources = append(cur.Sources, src)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return ch, nil
}

// Problem is a structural issue found without touching the network.
type Problem struct {
	Entry *Entry
	Msg   string
}

func (p Problem) String() string {
	return fmt.Sprintf("%s:%d: entry %s %q: %s", p.Entry.Chapter.File, p.Entry.Line, p.Entry.ID(), p.Entry.Title, p.Msg)
}

var numberRe = regexp.MustCompile(`\d+(?:[.,]\d+)*`)

// Lint checks the rules that need no network:
//   - Cost, Gain and Grade are present, Grade is A, B or C;
//   - there is at least one source, and every source has a quote;
//   - every number in the Gain line appears in one of the entry's quotes,
//     so a summary cannot claim a figure the sources don't contain.
func Lint(chs []*Chapter) []Problem {
	var out []Problem
	for _, ch := range chs {
		for _, e := range ch.Entries {
			add := func(f string, a ...any) { out = append(out, Problem{e, fmt.Sprintf(f, a...)}) }
			if e.Cost == "" {
				add("missing Cost")
			}
			if e.Gain == "" {
				add("missing Gain")
			}
			switch e.Grade {
			case "A", "B", "C":
			default:
				add("Grade must be A, B or C, got %q", e.Grade)
			}
			if len(e.Sources) == 0 {
				add("no Source")
			}
			var quotes strings.Builder
			for _, s := range e.Sources {
				if len(s.Quotes) == 0 {
					add("source %s has no quote", s.URL)
				}
				for _, q := range s.Quotes {
					quotes.WriteString(" " + q + " ")
				}
			}
			have := map[string]bool{}
			for _, n := range numberRe.FindAllString(quotes.String(), -1) {
				have[plainNumber(n)] = true
			}
			for _, n := range numberRe.FindAllString(e.Gain, -1) {
				if !have[plainNumber(n)] {
					add("number %s in Gain is not in any quote", n)
				}
			}
		}
	}
	return out
}

// plainNumber makes "10,000" and "10 000" style numbers comparable.
func plainNumber(s string) string {
	if strings.Count(s, ",") > 0 && !strings.Contains(s, ".") {
		return strings.ReplaceAll(s, ",", "")
	}
	return s
}

// Count returns entries per grade.
func Count(chs []*Chapter) (entries int, grades map[string]int, sources int, quotes int) {
	grades = map[string]int{}
	urls := map[string]bool{}
	for _, ch := range chs {
		for _, e := range ch.Entries {
			entries++
			grades[e.Grade]++
			for _, s := range e.Sources {
				urls[s.URL] = true
				quotes += len(s.Quotes)
			}
		}
	}
	return entries, grades, len(urls), quotes
}
