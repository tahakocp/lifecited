// Command cite lints, checks and builds the guide.
//
//	cite lint  [-book book]                    format and number rules, no network
//	cite check [-book book] [-cache dir] [-offline] [-strict] [-github] [-json file]
//	cite build [-book book] [-report file] [-o site]
//	cite stats [-book book]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tahakocp/lifecited/internal/book"
	"github.com/tahakocp/lifecited/internal/check"
	"github.com/tahakocp/lifecited/internal/site"
	"github.com/tahakocp/lifecited/internal/source"
)

const repoURL = "https://github.com/tahakocp/lifecited"

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd, args := os.Args[1], os.Args[2:]
	var code int
	switch cmd {
	case "lint":
		code = runLint(args)
	case "check":
		code = runCheck(args)
	case "build":
		code = runBuild(args)
	case "stats":
		code = runStats(args)
	default:
		usage()
	}
	os.Exit(code)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: cite lint|check|build|stats [flags]   (cite <cmd> -h for flags)")
	os.Exit(2)
}

func load(dir string) []*book.Chapter {
	chs, err := book.Load(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cite:", err)
		os.Exit(2)
	}
	return chs
}

func runLint(args []string) int {
	fs := flag.NewFlagSet("lint", flag.ExitOnError)
	dir := fs.String("book", "book", "chapter directory")
	gh := fs.Bool("github", false, "print GitHub Actions annotations")
	fs.Parse(args)
	chs := load(*dir)
	probs := book.Lint(chs)
	for _, p := range probs {
		if *gh {
			fmt.Printf("::error file=%s,line=%d::%s\n", filepath.Join(*dir, p.Entry.Chapter.File), p.Entry.Line, p.Msg)
		} else {
			fmt.Println(filepath.Join(*dir, p.String()))
		}
	}
	n, _, _, _ := book.Count(chs)
	fmt.Fprintf(os.Stderr, "%d entries, %d problems\n", n, len(probs))
	if len(probs) > 0 {
		return 1
	}
	return 0
}

func runCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	dir := fs.String("book", "book", "chapter directory")
	cache := fs.String("cache", "", "cache fetched pages in this directory")
	offline := fs.Bool("offline", false, "use only the cache")
	strict := fs.Bool("strict", false, "fail on unreachable pages too")
	gh := fs.Bool("github", false, "print GitHub Actions annotations")
	jsonOut := fs.String("json", "", "write the full report to this file")
	workers := fs.Int("j", 4, "parallel requests")
	fs.Parse(args)

	chs := load(*dir)
	probs := book.Lint(chs)
	for _, p := range probs {
		if *gh {
			fmt.Printf("::error file=%s,line=%d::%s\n", filepath.Join(*dir, p.Entry.Chapter.File), p.Entry.Line, p.Msg)
		} else {
			fmt.Println(filepath.Join(*dir, p.String()))
		}
	}
	f := source.NewFetcher(*cache)
	f.Offline = *offline
	t0 := time.Now()
	rep := check.Run(chs, f, *workers)

	warned := map[string]bool{}
	for _, r := range rep.Results {
		where := fmt.Sprintf("%s:%d", filepath.Join(*dir, r.Entry.Chapter.File), r.Source.Line)
		var msg string
		switch r.Status {
		case check.Missing:
			msg = fmt.Sprintf("quote not found on %s (first %d of %d letters match): %q", r.URL, r.Matched, r.Total, short(r.Quote))
		case check.Unreachable:
			msg = fmt.Sprintf("could not read %s: %s", r.URL, r.Err)
		}
		if r.Moved != "" && !warned[where+r.URL] {
			warned[where+r.URL] = true
			note := fmt.Sprintf("%s now redirects to %s; cite the new address", r.URL, r.Moved)
			if *gh {
				fmt.Printf("::warning file=%s,line=%d::%s\n", filepath.Join(*dir, r.Entry.Chapter.File), r.Source.Line, note)
			} else {
				fmt.Printf("%s: entry %s: warning: %s\n", where, r.EntryID, note)
			}
		}
		if msg == "" {
			continue
		}
		level := "error"
		if r.Status == check.Unreachable && !*strict {
			level = "warning"
		}
		if *gh {
			fmt.Printf("::%s file=%s,line=%d::entry %s: %s\n", level, filepath.Join(*dir, r.Entry.Chapter.File), r.Source.Line, r.EntryID, msg)
		} else {
			fmt.Printf("%s: entry %s: %s: %s\n", where, r.EntryID, level, msg)
		}
	}
	total := len(rep.Results)
	fmt.Fprintf(os.Stderr, "%d quotes on %d pages: %d found, %d missing, %d unreachable (%.1fs)\n",
		total, rep.Pages, rep.Counts[check.OK], rep.Counts[check.Missing], rep.Counts[check.Unreachable], time.Since(t0).Seconds())
	if *jsonOut != "" {
		b, _ := json.MarshalIndent(rep, "", " ")
		if err := os.WriteFile(*jsonOut, b, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "cite:", err)
			return 2
		}
	}
	if len(probs) > 0 {
		fmt.Fprintf(os.Stderr, "%d lint problems\n", len(probs))
	}
	if len(probs) > 0 || rep.Counts[check.Missing] > 0 || (*strict && rep.Counts[check.Unreachable] > 0) {
		return 1
	}
	return 0
}

func runBuild(args []string) int {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	dir := fs.String("book", "book", "chapter directory")
	out := fs.String("o", "site", "output directory")
	reportFile := fs.String("report", "", "check report (from cite check -json) to show per-quote status")
	fs.Parse(args)
	chs := load(*dir)
	if probs := book.Lint(chs); len(probs) > 0 {
		fmt.Fprintf(os.Stderr, "lint failed (%d problems); run cite lint\n", len(probs))
		return 1
	}
	opt := site.Options{Repo: repoURL, Now: time.Now()}
	if *reportFile != "" {
		b, err := os.ReadFile(*reportFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "cite:", err)
			return 2
		}
		var rep check.Report
		if err := json.Unmarshal(b, &rep); err != nil {
			fmt.Fprintln(os.Stderr, "cite: bad report:", err)
			return 2
		}
		opt.Report = &rep
	}
	html, err := site.Render(chs, opt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cite:", err)
		return 2
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "cite:", err)
		return 2
	}
	path := filepath.Join(*out, "index.html")
	if err := os.WriteFile(path, html, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "cite:", err)
		return 2
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d KB)\n", path, len(html)/1024)
	return 0
}

func runStats(args []string) int {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	dir := fs.String("book", "book", "chapter directory")
	fs.Parse(args)
	chs := load(*dir)
	n, g, pages, quotes := book.Count(chs)
	fmt.Printf("| chapter | entries | A | B | C |\n|---|---|---|---|---|\n")
	for _, ch := range chs {
		c := map[string]int{}
		for _, e := range ch.Entries {
			c[e.Grade]++
		}
		fmt.Printf("| [%d. %s](book/%s) | %d | %d | %d | %d |\n", ch.Number, ch.Title, ch.File, len(ch.Entries), c["A"], c["B"], c["C"])
	}
	fmt.Printf("| total | %d | %d | %d | %d |\n\n%d quotes from %d distinct pages.\n", n, g["A"], g["B"], g["C"], quotes, pages)
	return 0
}

func short(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 90 {
		return s[:87] + "..."
	}
	return s
}
