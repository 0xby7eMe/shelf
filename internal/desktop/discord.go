package desktop

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Activity is what Discord shows under the user's name.
type Activity struct {
	Details string // first line: the game
	State   string // second line: where it is from
	Start   int64  // unix seconds the session began; 0 for no timer
}

// Discord's local IPC: a unix socket, and frames of an opcode, a length (both
// little endian uint32) and a JSON body.
const (
	opHandshake = 0
	opFrame     = 1
	opClose     = 2

	retryEvery = 15 * time.Second
	ioTimeout  = 3 * time.Second
)

// Presence shows one Activity at a time. Discord may be closed or start later,
// so Presence remembers what it wants to show and keeps trying to connect.
type Presence struct {
	mu       sync.Mutex
	clientID string
	conn     net.Conn
	want     *Activity
	sockets  func() []string
	stop     chan struct{}
}

func NewPresence() *Presence {
	return &Presence{sockets: socketCandidates}
}

// socketCandidates lists where Discord (native, Flatpak or Snap) puts its socket.
func socketCandidates() []string {
	var dirs []string
	for _, v := range []string{"XDG_RUNTIME_DIR", "TMPDIR", "TMP", "TEMP"} {
		if d := os.Getenv(v); d != "" {
			dirs = append(dirs, d)
		}
	}
	dirs = append(dirs, "/tmp")

	subdirs := []string{
		"",
		"snap.discord",
		"app/com.discordapp.Discord",
		"app/com.discordapp.DiscordCanary",
		"app/dev.vencord.Vesktop",
		".flatpak/dev.vencord.Vesktop/xdg-run",
	}
	var out []string
	for _, d := range dirs {
		for _, sub := range subdirs {
			for i := 0; i < 10; i++ {
				out = append(out, filepath.Join(d, sub, "discord-ipc-"+strconv.Itoa(i)))
			}
		}
	}
	return out
}

// Enable starts presence for the given Discord application. Calling it again
// with another id reconnects.
func (p *Presence) Enable(clientID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked()
	p.clientID = clientID
	if p.stop == nil {
		p.stop = make(chan struct{})
		go p.retryLoop(p.stop)
	}
	p.sendLocked()
}

// Disable clears the presence and stops trying.
func (p *Presence) Disable() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stop != nil {
		close(p.stop)
		p.stop = nil
	}
	p.closeLocked()
	p.clientID = ""
}

// Set changes what is shown; nil clears it. Before Enable it only remembers.
func (p *Presence) Set(a *Activity) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.want = a
	p.sendLocked()
}

// Connected reports whether Discord is currently reachable.
func (p *Presence) Connected() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.conn != nil
}

func (p *Presence) retryLoop(stop chan struct{}) {
	t := time.NewTicker(retryEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			p.mu.Lock()
			if p.clientID != "" && p.conn == nil {
				p.sendLocked()
			}
			p.mu.Unlock()
		}
	}
}

// sendLocked connects if needed and pushes the wanted activity. A failure drops
// the connection; the retry loop tries again later.
func (p *Presence) sendLocked() {
	if p.clientID == "" {
		return
	}
	if p.conn == nil {
		conn, err := p.connectLocked()
		if err != nil {
			return
		}
		p.conn = conn
	}
	if err := setActivity(p.conn, p.want); err != nil {
		p.closeLocked()
	}
}

func (p *Presence) closeLocked() {
	if p.conn != nil {
		_ = writeFrame(p.conn, opClose, map[string]any{})
		p.conn.Close()
		p.conn = nil
	}
}

func (p *Presence) connectLocked() (net.Conn, error) {
	for _, path := range p.sockets() {
		conn, err := net.DialTimeout("unix", path, time.Second)
		if err != nil {
			continue
		}
		if err := handshake(conn, p.clientID); err != nil {
			conn.Close()
			continue
		}
		return conn, nil
	}
	return nil, fmt.Errorf("discord is not running")
}

func handshake(conn net.Conn, clientID string) error {
	if err := writeFrame(conn, opHandshake, map[string]any{"v": 1, "client_id": clientID}); err != nil {
		return err
	}
	op, body, err := readFrame(conn)
	if err != nil {
		return err
	}
	if op == opClose {
		return fmt.Errorf("discord refused: %s", body)
	}
	var ready struct {
		Evt string `json:"evt"`
	}
	if err := json.Unmarshal(body, &ready); err != nil || ready.Evt != "READY" {
		return fmt.Errorf("unexpected reply: %s", body)
	}
	return nil
}

func setActivity(conn net.Conn, a *Activity) error {
	args := map[string]any{"pid": os.Getpid()}
	if a != nil {
		act := map[string]any{"details": a.Details}
		if a.State != "" {
			act["state"] = a.State
		}
		if a.Start > 0 {
			act["timestamps"] = map[string]any{"start": a.Start}
		}
		args["activity"] = act
	}
	nonce := strconv.FormatInt(time.Now().UnixNano(), 36)
	err := writeFrame(conn, opFrame, map[string]any{"cmd": "SET_ACTIVITY", "args": args, "nonce": nonce})
	if err != nil {
		return err
	}
	// Discord answers every command; read it so the socket never fills up, and
	// notice when the answer is an error.
	op, body, err := readFrame(conn)
	if err != nil {
		return err
	}
	if op == opClose {
		return fmt.Errorf("discord closed the connection")
	}
	var reply struct {
		Evt  string `json:"evt"`
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &reply) == nil && reply.Evt == "ERROR" {
		return fmt.Errorf("discord: %s", reply.Data.Message)
	}
	return nil
}

func writeFrame(conn net.Conn, op uint32, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, op)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(body)))
	buf.Write(body)
	_ = conn.SetWriteDeadline(time.Now().Add(ioTimeout))
	_, err = conn.Write(buf.Bytes())
	return err
}

func readFrame(conn net.Conn) (uint32, []byte, error) {
	_ = conn.SetReadDeadline(time.Now().Add(ioTimeout))
	var head [8]byte
	if _, err := io.ReadFull(conn, head[:]); err != nil {
		return 0, nil, err
	}
	op := binary.LittleEndian.Uint32(head[:4])
	n := binary.LittleEndian.Uint32(head[4:])
	if n > 1<<20 {
		return 0, nil, fmt.Errorf("oversized frame")
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(conn, body); err != nil {
		return 0, nil, err
	}
	return op, body, nil
}
