package source

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTMLText(t *testing.T) {
	in := `<html><head><title>T</title><script>var x="hidden";</script></head>
<body><nav>Menu</nav><p>Hold the burn under cool running water for 20&nbsp;minutes.</p>
<!-- comment --><style>.a{}</style><ul><li>one</li><li>two &amp; three</li></ul></body></html>`
	got := HTMLText(in)
	for _, want := range []string{"Hold the burn under cool running water for 20 minutes.", "two & three", "Menu"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	for _, bad := range []string{"hidden", "comment", ".a{}", "<"} {
		if strings.Contains(got, bad) {
			t.Errorf("should not contain %q: %q", bad, got)
		}
	}
}

func TestPubmedID(t *testing.T) {
	if pubmedID("https://pubmed.ncbi.nlm.nih.gov/30703272/") != "30703272" {
		t.Error("pubmed id")
	}
	if pubmedID("https://pubmed.ncbi.nlm.nih.gov/?term=x") != "" {
		t.Error("search url is not an article")
	}
}

func TestFetchRedirectCacheOffline(t *testing.T) {
	var hits int
	mux := http.NewServeMux()
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/new", http.StatusMovedPermanently) })
	mux.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		hits++
		if ua := r.Header.Get("User-Agent"); ua != UserAgent {
			t.Errorf("user agent %q", ua)
		}
		fmt.Fprint(w, "<!doctype html><p>Fresh <b>text</b>.</p>")
	})
	mux.HandleFunc("/gone", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "no", http.StatusNotFound) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	dir := t.TempDir()
	f := NewFetcher(dir)
	p, err := f.Fetch(srv.URL + "/old")
	if err != nil {
		t.Fatal(err)
	}
	if p.Text != "Fresh text." || p.FinalURL != srv.URL+"/new" {
		t.Errorf("page %+v", p)
	}
	off := NewFetcher(dir)
	off.Offline = true
	p2, err := off.Fetch(srv.URL + "/old")
	if err != nil || p2.Text != p.Text || hits != 1 {
		t.Errorf("cache: %v %+v hits=%d", err, p2, hits)
	}
	if _, err := off.Fetch(srv.URL + "/other"); err != ErrNotCached {
		t.Errorf("offline miss: %v", err)
	}
	if _, err := f.Fetch(srv.URL + "/gone"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("404: %v", err)
	}
}
