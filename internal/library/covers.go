package library

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var appIDRe = regexp.MustCompile(`^\d{1,12}$`)
var coverKinds = []string{"library_600x900", "header"}
var heroKinds = []string{"library_hero"}

type Covers struct {
	dir    string
	client *http.Client

	mu    sync.Mutex
	locks map[string]*sync.Mutex 
	miss  map[string]bool        
}

func NewCovers() (*Covers, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(base, "shelf", "covers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Covers{
		dir:    dir,
		client: &http.Client{Timeout: 15 * time.Second},
		locks:  map[string]*sync.Mutex{},
		miss:   map[string]bool{},
	}, nil
}

func (c *Covers) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := strings.CutPrefix(r.URL.Path, "/cover/")
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		kinds := coverKinds
		if r.URL.Query().Get("kind") == "hero" {
			kinds = heroKinds
		}
		path, err := c.Find(r.Context(), id, kinds)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeFile(w, r, path)
	})
}

func (c *Covers) lockFor(id string) *sync.Mutex {
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.locks[id]
	if !ok {
		l = &sync.Mutex{}
		c.locks[id] = l
	}
	return l
}

func (c *Covers) Find(ctx context.Context, id string, kinds []string) (string, error) {
	if !appIDRe.MatchString(id) {
		return "", fmt.Errorf("invalid app id")
	}

	l := c.lockFor(id)
	l.Lock()
	defer l.Unlock()

	root, _ := findSteamRoot()

	for _, kind := range kinds {
		cached := filepath.Join(c.dir, id+"_"+kind+".jpg")
		if fileExists(cached) {
			return cached, nil
		}
		if root != "" {
			if p := localCover(root, id, kind); p != "" {
				return p, nil
			}
		}

		key := id + "/" + kind
		c.mu.Lock()
		missed := c.miss[key]
		c.mu.Unlock()
		if missed {
			continue
		}

		found, err := c.download(ctx, id, kind, cached)
		if err != nil {
			return "", err
		}
		if found {
			return cached, nil
		}
		c.mu.Lock()
		c.miss[key] = true
		c.mu.Unlock()
	}
	return "", fmt.Errorf("no cover for %s", id)
}

func localCover(root, id, kind string) string {
	lc := filepath.Join(root, "appcache", "librarycache")
	patterns := []string{
		filepath.Join(lc, id+"_"+kind+".jpg"),      
		filepath.Join(lc, id, kind+".jpg"),         
		filepath.Join(lc, id, "*", kind+".jpg"),    
	}
	for _, p := range patterns {
		if m, _ := filepath.Glob(p); len(m) > 0 {
			return m[0]
		}
	}
	return ""
}

func (c *Covers) download(ctx context.Context, id, kind, dest string) (bool, error) {
	url := fmt.Sprintf("https://cdn.cloudflare.steamstatic.com/steam/apps/%s/%s.jpg", id, kind)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("cdn returned %s", resp.Status)
	}

	tmp, err := os.CreateTemp(c.dir, "dl-*")
	if err != nil {
		return false, err
	}
	_, copyErr := io.Copy(tmp, resp.Body)
	closeErr := tmp.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(tmp.Name())
		if copyErr != nil {
			return false, copyErr
		}
		return false, closeErr
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		os.Remove(tmp.Name())
		return false, err
	}
	return true, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}