package library

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const maxCardImage = 8 << 20 // a poster is a few hundred KB; this is a safety cap

// DataURL returns a game's cover as a data: URL. The share card is drawn on a
// canvas, which can't be exported once it holds images from another origin, so
// the covers come through here instead.
func (c *Covers) DataURL(ctx context.Context, g Game) (string, error) {
	if g.Cover == "" {
		return "", fmt.Errorf("no cover for %s", g.ID)
	}

	if strings.HasPrefix(g.Cover, "/cover/") {
		path, err := c.Find(ctx, g.ExternalID, coverKinds)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return imageDataURL(data)
	}

	if !strings.HasPrefix(g.Cover, "https://") && !strings.HasPrefix(g.Cover, "http://") {
		return "", fmt.Errorf("unsupported cover for %s", g.ID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.Cover, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cover for %s: %s", g.ID, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxCardImage+1))
	if err != nil {
		return "", err
	}
	return imageDataURL(data)
}

func imageDataURL(data []byte) (string, error) {
	if len(data) == 0 || len(data) > maxCardImage {
		return "", fmt.Errorf("cover has an unusable size")
	}
	mime := http.DetectContentType(data)
	switch mime {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
	default:
		return "", fmt.Errorf("cover is %s, not an image", mime)
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

const pngDataPrefix = "data:image/png;base64,"

// DecodePNGDataURL turns the canvas export back into PNG bytes.
func DecodePNGDataURL(url string) ([]byte, error) {
	b64, ok := strings.CutPrefix(url, pngDataPrefix)
	if !ok {
		return nil, fmt.Errorf("not a PNG image")
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > 4*maxCardImage {
		return nil, fmt.Errorf("image has an unusable size")
	}
	if http.DetectContentType(data) != "image/png" {
		return nil, fmt.Errorf("not a PNG image")
	}
	return data, nil
}
