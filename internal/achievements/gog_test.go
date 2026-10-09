package achievements

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type noGalaxy struct{}

func (noGalaxy) Error() string  { return "no galaxy" }
func (noGalaxy) NoGalaxy() bool { return true }

type fakeGogSession struct {
	user string
	docs map[string]string // game key -> JSON
	err  error
	url  string
}

func (f *fakeGogSession) GogUserID() string { return f.user }
func (f *fakeGogSession) GogGameGet(_ context.Context, key string, address func(string) string, out any) error {
	f.url = address("client-" + key)
	if f.err != nil {
		return f.err
	}
	doc, ok := f.docs[key]
	if !ok {
		return noGalaxy{}
	}
	return json.Unmarshal([]byte(doc), out)
}

func TestGogAchievements(t *testing.T) {
	s := &fakeGogSession{user: "9", docs: map[string]string{
		"gog-42": `{"total_count":4,"items":[
			{"achievement_key":"old","name":"Old","description":"d","visible":true,"image_url_locked":"l1","image_url_unlocked":"u1","date_unlocked":"2020-01-02T03:04:05+0000","rarity":50.5},
			{"achievement_key":"new","name":"New","visible":true,"image_url_locked":"l2","image_url_unlocked":"u2","date_unlocked":"2024-06-01T10:00:00+00:00","rarity":10},
			{"achievement_key":"easy","name":"Easy","visible":true,"image_url_locked":"l3","image_url_unlocked":"u3","date_unlocked":null,"rarity":80},
			{"achievement_key":"secret","name":"","visible":false,"image_url_locked":"l4","date_unlocked":null}]}`,
		"gog-7": `{"total_count":0,"items":[]}`,
	}}
	g := NewGOG(s)

	p, err := g.Progress(context.Background(), "gog-42")
	if err != nil || p != (Progress{Unlocked: 2, Total: 4}) {
		t.Fatalf("progress = %+v, %v", p, err)
	}
	if s.url != gogGameplayBase+"/clients/client-gog-42/users/9/achievements" {
		t.Errorf("url = %s", s.url)
	}

	d, err := g.Detail(context.Background(), "gog-42")
	if err != nil || d.Source != "gog" || d.GameID != "gog-42" || d.Unlocked != 2 || d.Total != 4 {
		t.Fatalf("detail = %+v, %v", d, err)
	}
	var order []string
	for _, a := range d.Achievements {
		order = append(order, a.ID)
	}
	if got := strings.Join(order, " "); got != "new old easy secret" {
		t.Errorf("order = %s", got)
	}
	a := d.Achievements
	if a[0].Icon != "u2" || a[0].UnlockedAt != 1717236000 || a[0].Percent != 10 {
		t.Errorf("new = %+v", a[0])
	}
	if a[1].UnlockedAt != 1577934245 || a[1].Description != "d" {
		t.Errorf("old = %+v", a[1])
	}
	if a[2].Unlocked || a[2].Icon != "l3" {
		t.Errorf("easy = %+v", a[2])
	}
	if s := a[3]; !s.Hidden || s.Name != "secret" || s.Percent != -1 {
		t.Errorf("secret = %+v", s)
	}

	for _, key := range []string{"gog-7", "gog-8"} { // none, and no Galaxy at all
		if _, err := g.Progress(context.Background(), key); !errors.Is(err, ErrNone) {
			t.Errorf("%s: %v", key, err)
		}
	}
	s.err = errors.New("down")
	if _, err := g.Detail(context.Background(), "gog-42"); err == nil || errors.Is(err, ErrNone) {
		t.Errorf("a failure is not \"none\": %v", err)
	}

	s.user = ""
	if ok, why := g.Ready(); ok || !strings.Contains(why, "Sign in to GOG") {
		t.Errorf("signed out: %v %q", ok, why)
	}
}

func TestGogUnlockTime(t *testing.T) {
	str := func(s string) *string { return &s }
	cases := map[*string]int64{nil: 0, str(""): 0, str("2020-01-02T03:04:05Z"): 1577934245, str("yesterday"): -1}
	for in, want := range cases {
		if got := gogUnlockTime(in); got != want {
			t.Errorf("%v: %d, want %d", in, got, want)
		}
	}
}
