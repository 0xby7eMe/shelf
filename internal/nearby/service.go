package nearby

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/zeroconf/v2"
)

const (
	serviceType = "_shelf._tcp"
	domain      = "local."
	// Port is where Shelf serves its game list, when it is free; otherwise any port is used.
	Port        = 47630
	libraryPath = "/shelf/nearby/v1/library"

	pollEvery    = 20 * time.Second // how often a peer's list is checked for changes
	fetchTimeout = 5 * time.Second
	maxFailures  = 2       // failed fetches in a row before a peer counts as gone
	maxBody      = 8 << 20 // the largest game list read from a peer
	maxGames     = 50000
	maxPeers     = 64
	mdnsTTL      = 120 // seconds; also how often a peer that came back is noticed
	netCheck     = 30 * time.Second
)

// PeerInfo is another Shelf on the network and what you share with it.
type PeerInfo struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Host   string   `json:"host,omitempty"`
	Games  int      `json:"games"`
	Common []Shared `json:"common"`
}

type Snapshot struct {
	Settings Settings   `json:"settings"`
	Name     string     `json:"name"` // how others see you
	Running  bool       `json:"running"`
	Port     int        `json:"port,omitempty"`
	Message  string     `json:"message,omitempty"` // what went wrong, in words for the user
	Peers    []PeerInfo `json:"peers"`
	// Everyone is the games all peers have in common with you; only filled with two peers or more.
	Everyone []Shared `json:"everyone"`
}

// Entry is a Shelf found on the network.
type Entry struct {
	ID      string
	Session string // changes with every start of that Shelf
	Host    string
	Port    int
	Addrs   []net.IP
}

type peer struct {
	Entry
	addr     string // host:port that answered last
	lib      *Library
	etag     string
	failures int
	wake     chan struct{}
	cancel   context.CancelFunc
}

// Service announces this Shelf, finds the others and keeps their game lists.
type Service struct {
	store *Store
	emit  func(event string, data any)

	// The network, swapped out by tests.
	listen   func() (net.Listener, error)
	announce func(port int, txt []string, session string) (stop func(), err error)
	browse   func(ctx context.Context, found func(Entry)) error
	netState func() string
	allow    func(net.IP) bool
	client   *http.Client

	mu      sync.Mutex
	parent  context.Context
	stop    context.CancelFunc
	done    chan struct{}
	port    int
	message string
	mine    []Game
	body    []byte
	etag    string
	peers   map[string]*peer
	greeted map[string]bool // peers "nearby:found" was sent for
}

// New makes the service; it does nothing until Start. emit sends events to the
// interface: "nearby:changed" when the peers or their lists change, and
// "nearby:found" with a PeerInfo the first time a peer shows up.
func New(store *Store, emit func(event string, data any)) *Service {
	return &Service{
		store:    store,
		emit:     emit,
		listen:   listenTCP,
		announce: mdnsAnnounce,
		browse:   mdnsBrowse,
		netState: interfaceState,
		allow:    lanAddr,
		client:   &http.Client{Timeout: fetchTimeout},
		peers:    map[string]*peer{},
		greeted:  map[string]bool{},
	}
}

// Start runs the service until ctx ends, if it is switched on.
func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	s.parent = ctx
	s.mu.Unlock()
	if s.store.Get().Enabled {
		s.start()
	}
}

func (s *Service) start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stop != nil || s.parent == nil {
		return
	}
	ln, err := s.listen()
	if err != nil {
		s.message = fmt.Sprintf("Couldn't open a port for the other Shelfs: %v", err)
		log.Printf("nearby: %v", err)
		return
	}
	s.message = ""
	s.port = ln.Addr().(*net.TCPAddr).Port

	ctx, cancel := context.WithCancel(s.parent)
	done := make(chan struct{})
	s.stop, s.done = cancel, done
	srv := &http.Server{Handler: http.HandlerFunc(s.serve), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("nearby: %v", err)
		}
	}()
	go func() {
		defer close(done)
		s.run(ctx, s.port)
		srv.Close()
	}()
}

// Stop takes this Shelf off the network and forgets the peers.
func (s *Service) Stop() {
	s.mu.Lock()
	stop, done := s.stop, s.done
	s.stop, s.done = nil, nil
	s.mu.Unlock()
	if stop == nil {
		return
	}
	stop()
	<-done

	s.mu.Lock()
	for _, p := range s.peers {
		p.cancel()
	}
	s.peers = map[string]*peer{}
	s.port = 0
	s.mu.Unlock()
	s.changed()
}

// run announces this Shelf and looks for others, and starts over when the
// network changes (another Wi-Fi, a new address), which mDNS doesn't follow.
func (s *Service) run(ctx context.Context, port int) {
	txt := []string{"id=" + s.store.ID(), "v=1"}
	session := newSession()
	for {
		mctx, cancel := context.WithCancel(ctx)
		state := s.netState()

		stopAnnounce, err := s.announce(port, txt, session)
		s.setMessage(err, "Couldn't announce Shelf on the network")
		go func() {
			if err := s.browse(mctx, s.found); err != nil {
				log.Printf("nearby: browse: %v", err)
			}
		}()

		changed := false
		ticker := time.NewTicker(netCheck)
		for !changed && ctx.Err() == nil {
			select {
			case <-ctx.Done():
			case <-ticker.C:
				changed = s.netState() != state
			}
		}
		ticker.Stop()
		cancel()
		if stopAnnounce != nil {
			stopAnnounce()
		}
		if ctx.Err() != nil {
			return
		}
		log.Printf("nearby: network changed, announcing again")
		s.mu.Lock()
		for _, p := range s.peers {
			p.poke()
		}
		s.mu.Unlock()
	}
}

func (s *Service) setMessage(err error, what string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		log.Printf("nearby: %s: %v", strings.ToLower(what[:1])+what[1:], err)
		s.message = fmt.Sprintf("%s: %v", what, err)
	} else {
		s.message = ""
	}
}

// found is called for every announcement seen; it must not block.
func (s *Service) found(e Entry) {
	if e.ID == "" || e.ID == s.store.ID() || e.Port <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stop == nil {
		return
	}
	if p, ok := s.peers[e.ID]; ok {
		if p.Session == e.Session && p.Port == e.Port {
			p.Addrs = e.Addrs
			p.Host = e.Host
			return
		}
		// The same Shelf started again, maybe on another port.
		p.cancel()
		delete(s.peers, e.ID)
	}
	if len(s.peers) >= maxPeers {
		return
	}
	ctx, cancel := context.WithCancel(s.parent)
	p := &peer{Entry: e, wake: make(chan struct{}, 1), cancel: cancel}
	s.peers[e.ID] = p
	go s.poll(ctx, p)
}

func (p *peer) poke() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// poll keeps a peer's list up to date until the peer stops answering.
func (s *Service) poll(ctx context.Context, p *peer) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		case <-p.wake:
		}

		lib, etag, addr, err := s.fetch(ctx, p)
		if ctx.Err() != nil {
			return
		}

		s.mu.Lock()
		if s.peers[p.ID] != p {
			s.mu.Unlock()
			return
		}
		var greet *PeerInfo
		changed := false
		if err != nil {
			p.failures++
			if p.failures >= maxFailures {
				delete(s.peers, p.ID)
				p.cancel()
				s.mu.Unlock()
				if p.lib != nil {
					s.changed()
				}
				return
			}
		} else {
			p.failures = 0
			p.addr = addr
			if lib != nil {
				p.lib, p.etag = lib, etag
				changed = true
				if !s.greeted[p.ID] {
					s.greeted[p.ID] = true
					info := s.peerInfoLocked(p)
					greet = &info
				}
			}
		}
		s.mu.Unlock()

		if changed {
			s.changed()
		}
		if greet != nil && s.emit != nil {
			s.emit("nearby:found", *greet)
		}
		timer.Reset(pollEvery)
	}
}

// fetch asks a peer for its list. A nil list with no error means it hasn't changed.
func (s *Service) fetch(ctx context.Context, p *peer) (*Library, string, string, error) {
	s.mu.Lock()
	addrs := []string{}
	if p.addr != "" {
		addrs = append(addrs, p.addr)
	}
	for _, ip := range p.Addrs {
		if s.allow(ip) {
			if a := net.JoinHostPort(ip.String(), strconv.Itoa(p.Port)); a != p.addr {
				addrs = append(addrs, a)
			}
		}
	}
	etag := p.etag
	s.mu.Unlock()

	if len(addrs) == 0 {
		return nil, "", "", errors.New("no address on the local network")
	}
	var err error
	for _, a := range addrs {
		var lib *Library
		var tag string
		lib, tag, err = s.fetchFrom(ctx, a, etag)
		if err == nil {
			if lib != nil && lib.ID != p.ID {
				err = fmt.Errorf("%s answered for another Shelf", a)
				continue
			}
			return lib, tag, a, nil
		}
	}
	return nil, "", "", err
}

func (s *Service) fetchFrom(ctx context.Context, addr, etag string) (*Library, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+libraryPath, nil)
	if err != nil {
		return nil, "", err
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotModified:
		return nil, etag, nil
	case http.StatusOK:
	default:
		return nil, "", fmt.Errorf("%s: %s", addr, resp.Status)
	}
	var lib Library
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&lib); err != nil {
		return nil, "", fmt.Errorf("%s: %w", addr, err)
	}
	lib.Name = cleanName(lib.Name)
	if lib.Name == "" {
		lib.Name = "Shelf"
	}
	if len(lib.Games) > maxGames {
		lib.Games = lib.Games[:maxGames]
	}
	return &lib, resp.Header.Get("ETag"), nil
}

// serve answers the other Shelfs, and only those on the local network.
func (s *Service) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != libraryPath {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if ip := net.ParseIP(host); err != nil || ip == nil || !(ip.IsLoopback() || lanAddr(ip) || ip.IsLinkLocalUnicast()) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	s.mu.Lock()
	body, etag := s.body, s.etag
	s.mu.Unlock()
	if body == nil {
		body, etag = s.document()
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// document builds the list served to others and keeps it until something changes.
func (s *Service) document() ([]byte, string) {
	s.mu.Lock()
	games := s.mine
	s.mu.Unlock()
	if games == nil {
		games = []Game{}
	}
	body, _ := json.Marshal(Library{ID: s.store.ID(), Name: s.store.DisplayName(), Games: games})
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`

	s.mu.Lock()
	s.body, s.etag = body, etag
	s.mu.Unlock()
	return body, etag
}

// SetLibrary is your own list of games, to serve and to compare with.
func (s *Service) SetLibrary(games []Game) {
	s.mu.Lock()
	same := equalGames(s.mine, games)
	if !same {
		s.mine = games
		s.body = nil
	}
	hasPeers := len(s.peers) > 0
	s.mu.Unlock()
	if !same && hasPeers {
		s.changed()
	}
}

func equalGames(a, b []Game) bool {
	if a == nil || len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// SetSettings saves the settings and switches the service on or off to match.
func (s *Service) SetSettings(next Settings) (Snapshot, error) {
	prev := s.store.Get()
	if err := s.store.Set(next); err != nil {
		return s.Snapshot(), err
	}
	s.mu.Lock()
	s.body = nil // the name may have changed
	s.mu.Unlock()
	switch {
	case next.Enabled && !prev.Enabled:
		s.start()
		s.changed()
	case !next.Enabled && prev.Enabled:
		s.Stop()
	}
	return s.Snapshot(), nil
}

// Snapshot is who is around and what you have in common with each of them.
func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := Snapshot{
		Settings: s.store.Get(),
		Name:     s.store.DisplayName(),
		Running:  s.stop != nil,
		Port:     s.port,
		Message:  s.message,
		Peers:    []PeerInfo{},
		Everyone: []Shared{},
	}
	var lists [][]Shared
	for _, p := range s.peers {
		if p.lib == nil {
			continue
		}
		info := s.peerInfoLocked(p)
		snap.Peers = append(snap.Peers, info)
		lists = append(lists, info.Common)
	}
	sort.Slice(snap.Peers, func(i, j int) bool {
		a, b := snap.Peers[i], snap.Peers[j]
		if la, lb := strings.ToLower(a.Name), strings.ToLower(b.Name); la != lb {
			return la < lb
		}
		return a.ID < b.ID
	})
	snap.Everyone = Everyone(lists)
	return snap
}

func (s *Service) peerInfoLocked(p *peer) PeerInfo {
	return PeerInfo{
		ID:     p.ID,
		Name:   p.lib.Name,
		Host:   p.Host,
		Games:  len(p.lib.Games),
		Common: Common(s.mine, p.lib.Games),
	}
}

func (s *Service) changed() {
	if s.emit != nil {
		s.emit("nearby:changed", nil)
	}
}

// --- the network ---

func listenTCP() (net.Listener, error) {
	if ln, err := net.Listen("tcp", ":"+strconv.Itoa(Port)); err == nil {
		return ln, nil
	}
	return net.Listen("tcp", ":0")
}

func mdnsAnnounce(port int, txt []string, session string) (func(), error) {
	srv, err := zeroconf.Register("shelf-"+session, serviceType, domain, port, txt, nil, zeroconf.TTL(mdnsTTL))
	if err != nil {
		return nil, err
	}
	return srv.Shutdown, nil
}

// mdnsBrowse reports every Shelf announced on the network until ctx ends,
// trying again while there is no network to browse.
func mdnsBrowse(ctx context.Context, found func(Entry)) error {
	for {
		entries := make(chan *zeroconf.ServiceEntry)
		returned := make(chan struct{})
		go func() {
			for {
				select {
				case e, ok := <-entries:
					if !ok {
						return
					}
					found(toEntry(e))
				case <-returned:
					// Browse only returns once nothing sends any more.
					return
				}
			}
		}()
		err := zeroconf.Browse(ctx, serviceType, domain, entries)
		close(returned)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			log.Printf("nearby: browse: %v", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(netCheck):
		}
	}
}

func toEntry(e *zeroconf.ServiceEntry) Entry {
	out := Entry{
		Session: e.Instance,
		Host:    strings.TrimSuffix(strings.TrimSuffix(e.HostName, "."), ".local"),
		Port:    e.Port,
	}
	for _, t := range e.Text {
		if v, ok := strings.CutPrefix(t, "id="); ok {
			out.ID = v
		}
	}
	out.Addrs = append(out.Addrs, e.AddrIPv4...)
	out.Addrs = append(out.Addrs, e.AddrIPv6...)
	return out
}

// interfaceState sums up the network addresses, to notice when they change.
func interfaceState() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	var parts []string
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagMulticast == 0 {
			continue
		}
		addrs, _ := ifi.Addrs()
		for _, a := range addrs {
			parts = append(parts, ifi.Name+"="+a.String())
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// lanAddr is an address a peer may be reached at: on a private network, and
// not one (such as an IPv6 link-local address) that needs an interface named.
func lanAddr(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		return ip4.IsPrivate() || ip4.IsLinkLocalUnicast()
	}
	return ip.IsPrivate()
}

func newSession() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}
