package friends

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
)

// fakeGogSession answers GOG's friends and presence services from canned JSON.
type fakeGogSession struct {
	user     string
	docs     map[string]string // URL prefix -> JSON
	asked    []string
	titles   map[string]string
	failWith error
}

func (f *fakeGogSession) GogUserID() string          { return f.user }
func (f *fakeGogSession) GogTitle(key string) string { return f.titles[key] }
func (f *fakeGogSession) GogGet(_ context.Context, u string, out any) error {
	f.asked = append(f.asked, u)
	for prefix, doc := range f.docs {
		if strings.HasPrefix(u, prefix) {
			if strings.Contains(prefix, "presence") && f.failWith != nil {
				return f.failWith
			}
			return json.Unmarshal([]byte(doc), out)
		}
	}
	return errors.New("unexpected " + u)
}

func TestGogFriends(t *testing.T) {
	s := &fakeGogSession{
		user: "9",
		docs: map[string]string{
			gogChatBase + "/users/9/friends": `{"items":[
				{"user_id":"101","username":"ciri","images":{"medium":"m.png","medium_2x":"m2.png"}},
				{"user_id":"102","username":"yen","images":{"medium":"y.png"}},
				{"user_id":"103","username":"jaskier"},
				{"user_id":"104","username":"zoltan"},
				{"user_id":"","username":"ghost"}]}`,
			gogPresenceBase + "/statuses": `{"total_count":3,"items":[
				{"user_id":"101","client_id":"x","data":{"presence":"online","game_id":"1207658924"}},
				{"user_id":"102","data":{"presence":"online","game_id":1207664643}},
				{"user_id":"103","data":{}},
				{"user_id":"104","data":{"presence":"offline"}},
				{"user_id":"999","data":{"presence":"online"}}]}`,
		},
		titles: map[string]string{"gog-1207658924": "The Witcher"},
	}
	g := NewGOG(s)
	if ok, _ := g.Ready(); !ok {
		t.Fatal("ready when signed in")
	}
	list, err := g.Friends(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 4 {
		t.Fatalf("friends = %+v", list)
	}
	by := map[string]Friend{}
	for _, f := range list {
		by[f.Name] = f
	}
	if c := by["ciri"]; c.Status != StatusPlaying || c.Playing == nil || *c.Playing != (Playing{Source: "gog", ID: "gog-1207658924", Name: "The Witcher"}) ||
		c.Avatar != "m2.png" || c.ProfileURL != "https://www.gog.com/u/ciri" {
		t.Errorf("ciri = %+v %+v", c, c.Playing)
	}
	if y := by["yen"]; y.Status != StatusPlaying || y.Playing.ID != "gog-1207664643" || y.Playing.Name != "a game" || y.Avatar != "y.png" {
		t.Errorf("a game outside the library, sent as a number: %+v %+v", y, y.Playing)
	}
	if j := by["jaskier"]; j.Status != StatusOnline || j.Playing != nil {
		t.Errorf("listed without details is online: %+v", j)
	}
	if z := by["zoltan"]; z.Status != StatusOffline {
		t.Errorf("zoltan = %+v", z)
	}
	if q := s.asked[1]; q != gogPresenceBase+"/statuses?user_id=101,102,103,104" {
		t.Errorf("presence asked = %s", q)
	}

	s.failWith = errors.New("down")
	if _, err := g.Friends(context.Background()); err == nil || !strings.Contains(err.Error(), "who is online") {
		t.Errorf("a presence failure is not everyone offline: %v", err)
	}

	s.user = ""
	if ok, why := g.Ready(); ok || !strings.Contains(why, "Sign in to GOG") {
		t.Errorf("signed out: %v %q", ok, why)
	}
}

func TestGogFriendsAskPresenceInBatches(t *testing.T) {
	var items []string
	for i := range 230 {
		items = append(items, `{"user_id":"`+strconv.Itoa(1000+i)+`","username":"u`+strconv.Itoa(i)+`"}`)
	}
	s := &fakeGogSession{user: "9", docs: map[string]string{
		gogChatBase:     `{"items":[` + strings.Join(items, ",") + `]}`,
		gogPresenceBase: `{"items":[]}`,
	}}
	if list, err := NewGOG(s).Friends(context.Background()); err != nil || len(list) != 230 {
		t.Fatalf("%d, %v", len(list), err)
	}
	if len(s.asked) != 1+3 {
		t.Errorf("requests = %d", len(s.asked))
	}
}

func TestGogGameID(t *testing.T) {
	for in, want := range map[any]string{"42": "42", " 42 ": "42", 42.0: "42", 0.0: "", "0": "", "4a": "", 1.5: "", nil: "", true: ""} {
		if got := gogGameID(in); got != want {
			t.Errorf("gogGameID(%v) = %q, want %q", in, got, want)
		}
	}
}
