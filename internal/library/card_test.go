package library

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDataURLFetchesRemoteCovers(t *testing.T) {
	img := tinyPNG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.png":
			w.Write(img)
		case "/text":
			w.Write([]byte("<html>not an image</html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Covers{client: &http.Client{Timeout: time.Second}}
	ctx := context.Background()

	url, err := c.DataURL(ctx, Game{ID: "epic:a", Cover: srv.URL + "/ok.png"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(img); url != want {
		t.Errorf("got %q", url)
	}

	for _, path := range []string{"/text", "/missing"} {
		if _, err := c.DataURL(ctx, Game{ID: "epic:a", Cover: srv.URL + path}); err == nil {
			t.Errorf("%s: want an error", path)
		}
	}
	for _, cover := range []string{"", "file:///etc/passwd", "ftp://x/y.png"} {
		if _, err := c.DataURL(ctx, Game{ID: "epic:a", Cover: cover}); err == nil {
			t.Errorf("cover %q: want an error", cover)
		}
	}
}

func TestDecodePNGDataURL(t *testing.T) {
	img := tinyPNG(t)
	good := pngDataPrefix + base64.StdEncoding.EncodeToString(img)
	got, err := DecodePNGDataURL(good)
	if err != nil || !bytes.Equal(got, img) {
		t.Fatalf("round trip failed: %v", err)
	}

	bad := []string{
		"",
		"data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte("<script>")),
		pngDataPrefix + "!!!",
		pngDataPrefix + base64.StdEncoding.EncodeToString([]byte("not a png")),
		strings.Repeat("x", 10),
	}
	for _, b := range bad {
		if _, err := DecodePNGDataURL(b); err == nil {
			t.Errorf("%.30q: want an error", b)
		}
	}
}
