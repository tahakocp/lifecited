// Package source fetches a cited page and turns it into plain text that
// quotes can be matched against.
package source

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// UserAgent is sent with every request. Some sites (cdc.gov, for one) refuse
// browser-looking agents from data centres but accept an honest one.
const UserAgent = "lifecited-check/0.1 (+https://github.com/tahakocp/lifecited)"

// Fetcher downloads sources, optionally through an on-disk cache.
type Fetcher struct {
	Client   *http.Client
	CacheDir string // empty: no cache
	Offline  bool   // only read the cache
}

func NewFetcher(cacheDir string) *Fetcher {
	return &Fetcher{Client: &http.Client{Timeout: 40 * time.Second}, CacheDir: cacheDir}
}

var ErrNotCached = errors.New("not in cache (offline)")

// Page is what was read for one cited URL.
type Page struct {
	URL      string    `json:"url"`
	FinalURL string    `json:"final_url"` // after redirects
	Text     string    `json:"text"`
	Fetched  time.Time `json:"fetched"`
}

// Fetch returns the readable text of the page at rawURL.
func (f *Fetcher) Fetch(rawURL string) (*Page, error) {
	key := cacheKey(rawURL)
	if f.CacheDir != "" {
		if b, err := os.ReadFile(filepath.Join(f.CacheDir, key)); err == nil {
			var p Page
			if json.Unmarshal(b, &p) == nil && p.URL == rawURL {
				return &p, nil
			}
		}
	}
	if f.Offline {
		return nil, ErrNotCached
	}
	p := &Page{URL: rawURL, FinalURL: rawURL, Fetched: time.Now().UTC()}
	if pmid := pubmedID(rawURL); pmid != "" {
		text, err := f.pubmed(pmid)
		if err != nil {
			return nil, err
		}
		p.Text = text
	} else {
		body, ctype, final, err := f.get(rawURL)
		if err != nil {
			return nil, err
		}
		p.FinalURL = final
		if strings.Contains(ctype, "html") || looksHTML(body) {
			p.Text = HTMLText(string(body))
		} else {
			p.Text = string(body)
		}
	}
	if f.CacheDir != "" {
		if b, err := json.Marshal(p); err == nil {
			_ = os.MkdirAll(f.CacheDir, 0o755)
			_ = os.WriteFile(filepath.Join(f.CacheDir, key), b, 0o644)
		}
	}
	return p, nil
}

func (f *Fetcher) get(u string) ([]byte, string, string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			return nil, "", "", err
		}
		req.Header.Set("User-Agent", UserAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.9,*/*;q=0.5")
		req.Header.Set("Accept-Language", "en")
		resp, err := f.Client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			if resp.StatusCode == 404 || resp.StatusCode == 410 {
				break
			}
			continue
		}
		return body, resp.Header.Get("Content-Type"), resp.Request.URL.String(), nil
	}
	return nil, "", "", lastErr
}

var pubmedRe = regexp.MustCompile(`^https://pubmed\.ncbi\.nlm\.nih\.gov/(\d+)/?$`)

func pubmedID(u string) string {
	if m := pubmedRe.FindStringSubmatch(u); m != nil {
		return m[1]
	}
	return ""
}

// pubmed reads title and abstract through the Europe PMC REST API, because
// pubmed.ncbi.nlm.nih.gov answers scripted requests with a challenge page.
func (f *Fetcher) pubmed(pmid string) (string, error) {
	api := "https://www.ebi.ac.uk/europepmc/webservices/rest/search?resultType=core&format=json&query=" +
		url.QueryEscape("EXT_ID:"+pmid+" AND SRC:MED")
	body, _, _, err := f.get(api)
	if err != nil {
		return "", err
	}
	var r struct {
		ResultList struct {
			Result []struct {
				Title        string `json:"title"`
				AbstractText string `json:"abstractText"`
			} `json:"result"`
		} `json:"resultList"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}
	if len(r.ResultList.Result) == 0 {
		return "", fmt.Errorf("PMID %s not found in Europe PMC", pmid)
	}
	a := r.ResultList.Result[0]
	return HTMLText(a.Title + "\n" + a.AbstractText), nil
}

func looksHTML(b []byte) bool {
	head := strings.ToLower(string(b[:min(len(b), 512)]))
	return strings.Contains(head, "<html") || strings.Contains(head, "<!doctype html")
}

func cacheKey(u string) string {
	h := sha256.Sum256([]byte(u))
	return hex.EncodeToString(h[:8]) + ".json"
}

var (
	dropBlocks = regexp.MustCompile(`(?is)<(script|style|noscript|svg|template|head)\b.*?</(script|style|noscript|svg|template|head)\s*>`)
	comments   = regexp.MustCompile(`(?s)<!--.*?-->`)
	blockTags  = regexp.MustCompile(`(?i)</?(p|div|li|ul|ol|h[1-6]|br|tr|td|th|table|section|article|header|footer|nav|blockquote|dd|dt)\b[^>]*>`)
	anyTag     = regexp.MustCompile(`(?s)<[^>]*>`)
	spaces     = regexp.MustCompile(`[ \t\r\f\v]+`)
	blankLines = regexp.MustCompile(`\n\s*\n+`)
)

// HTMLText strips markup and returns the visible text, one block per line.
func HTMLText(s string) string {
	s = comments.ReplaceAllString(s, " ")
	s = dropBlocks.ReplaceAllString(s, " ")
	s = blockTags.ReplaceAllString(s, "\n")
	s = anyTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = spaces.ReplaceAllString(s, " ")
	s = blankLines.ReplaceAllString(s, "\n")
	return strings.TrimSpace(s)
}
