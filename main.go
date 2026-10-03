package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

type selection struct {
	Name      string   `json:"name"`
	Character string   `json:"character"`
	Spells    []string `json:"spells"`
}
type command struct {
	Slot     string `json:"slot"`
	Target   string `json:"target"`
	Position *point `json:"position"`
	Sequence uint64 `json:"sequence"`
	Type     string `json:"type"`
	MatchID  string `json:"matchId"`
	selection
}
type player struct {
	ID    string `json:"id"`
	Team  string `json:"team"`
	Ready bool   `json:"ready"`
	selection
}
type match struct {
	Game     *game     `json:"-"`
	ID       string    `json:"id"`
	Phase    string    `json:"phase"`
	Players  [2]player `json:"players"`
	Deadline int64     `json:"deadline"`
}
type session struct {
	id          string
	connections map[*client]bool
	queued      bool
	choice      selection
	matchID     string
	seen        time.Time
}
type client struct {
	conn    *websocket.Conn
	send    chan []byte
	session *session
}
type lobby struct {
	mu       sync.Mutex
	sessions map[string]*session
	matches  map[string]*match
	queue    []*session
}

func newLobby() *lobby { return &lobby{sessions: map[string]*session{}, matches: map[string]*match{}} }
func randomID() string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func validSelection(s selection) bool {
	if !utf8.ValidString(s.Name) || utf8.RuneCountInString(s.Name) < 1 || utf8.RuneCountInString(s.Name) > 16 {
		return false
	}
	for _, c := range s.Name {
		if unicode.IsControl(c) {
			return false
		}
	}
	switch s.Character {
	case "Sophie", "Jude", "Nadia", "Chiyo":
	default:
		return false
	}
	if len(s.Spells) != 2 || s.Spells[0] == s.Spells[1] {
		return false
	}
	for _, v := range s.Spells {
		if v != "flash" && v != "ignite" && v != "barrier" {
			return false
		}
	}
	return true
}
func (l *lobby) originOK(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Host == r.Host && (u.Scheme == "http" || u.Scheme == "https") {
		return true
	}
	for _, allowed := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		if origin == strings.TrimSpace(allowed) {
			return true
		}
	}
	return false
}
func (l *lobby) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/session", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		if !l.originOK(r) {
			http.Error(w, "origin forbidden", 403)
			return
		}
		l.mu.Lock()
		defer l.mu.Unlock()
		var token string
		if c, err := r.Cookie("auxilia_moba_session"); err == nil {
			token = c.Value
		}
		s := l.sessions[token]
		if s == nil {
			token = randomID()
			s = &session{id: randomID(), connections: map[*client]bool{}}
			l.sessions[token] = s
		}
		s.seen = time.Now()
		http.SetCookie(w, &http.Cookie{Name: "auxilia_moba_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil || os.Getenv("COOKIE_SECURE") == "true", MaxAge: 86400})
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(map[string]string{"id": s.id})
	})
	mux.HandleFunc("/ws", l.serveWS)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	return mux
}

// All mutations and broadcasts share a lock, including queue/cancel races.
func (l *lobby) emit(c *client, v any) {
	b, _ := json.Marshal(v)
	select {
	case c.send <- b:
	default:
		c.conn.Close()
	}
}
func (l *lobby) active() int {
	n := 0
	for _, s := range l.sessions {
		if len(s.connections) > 0 {
			n++
		}
	}
	return n
}
func (l *lobby) state(s *session, notice string) {
	phase := "entrance"
	if s.queued {
		phase = "queued"
	}
	m := l.matches[s.matchID]
	if m != nil {
		phase = m.Phase
	}
	for c := range s.connections {
		l.emit(c, map[string]any{"type": "state", "selfId": s.id, "phase": phase, "selection": s.choice, "match": m, "notice": notice})
	}
	if m != nil && m.Phase == "playing" {
		l.snapshot(s, m)
	}
}
func (l *lobby) presence() {
	n := l.active()
	for _, s := range l.sessions {
		for c := range s.connections {
			l.emit(c, map[string]any{"type": "presence", "active": n})
		}
	}
}
func (l *lobby) removeQueue(s *session) {
	s.queued = false
	for i, v := range l.queue {
		if v == s {
			l.queue = append(l.queue[:i], l.queue[i+1:]...)
			break
		}
	}
}
func (l *lobby) matchState(m *match) {
	for _, s := range l.sessions {
		if s.matchID == m.ID {
			l.state(s, "")
		}
	}
}
func (l *lobby) finish(m *match, notice string) {
	delete(l.matches, m.ID)
	for _, s := range l.sessions {
		if s.matchID == m.ID {
			s.matchID = ""
			l.state(s, notice)
		}
	}
}
func (l *lobby) handle(c *client, cmd command, now time.Time) {
	s := c.session
	s.seen = now
	fail := func(message string) { l.emit(c, map[string]string{"type": "error", "message": message}) }
	switch cmd.Type {
	case "move", "stop", "recall", "attack", "cast", "spell", "upgrade":
		l.gameInput(c, cmd)
	case "queue":
		if s.queued || s.matchID != "" {
			l.state(s, "")
			return
		}
		cmd.Name = strings.TrimSpace(cmd.Name)
		if !validSelection(cmd.selection) {
			fail("名前・キャラクター・異なる2つのスペルを確認してください。")
			return
		}
		s.choice = cmd.selection
		s.queued = true
		l.queue = append(l.queue, s)
		l.state(s, "")
		if len(l.queue) >= 2 {
			a, b := l.queue[0], l.queue[1]
			l.queue = l.queue[2:]
			a.queued = false
			b.queued = false
			var coin [1]byte
			if _, err := rand.Read(coin[:]); err != nil {
				panic(err)
			}
			if coin[0]&1 == 1 {
				a, b = b, a
			}
			m := &match{ID: randomID(), Phase: "loading", Deadline: now.Add(30 * time.Second).UnixMilli(), Players: [2]player{{ID: a.id, Team: "blue", selection: a.choice}, {ID: b.id, Team: "red", selection: b.choice}}}
			l.matches[m.ID] = m
			a.matchID = m.ID
			b.matchID = m.ID
			l.matchState(m)
		}
	case "cancel":
		if s.matchID != "" {
			fail("先にマッチングが成立しました。退出する場合は成立画面から戻ってください。")
			l.state(s, "")
			return
		}
		l.removeQueue(s)
		l.state(s, "")
	case "ready", "leave":
		m := l.matches[s.matchID]
		if m == nil || cmd.MatchID != m.ID {
			fail("現在のマッチが見つかりません。")
			return
		}
		if cmd.Type == "leave" {
			if m.Phase == "playing" {
				winner := "相手"
				for _, p := range m.Players {
					if p.ID != s.id {
						winner = p.Name
					}
				}
				l.finish(m, winner+"の勝利（相手が退出）")
			} else {
				l.finish(m, "プレイヤーが退出したため、マッチを終了しました。")
			}
			return
		}
		if m.Phase != "loading" {
			l.state(s, "")
			return
		}
		if now.UnixMilli() >= m.Deadline {
			l.finish(m, "準備が30秒以内に完了しなかったため、マッチを無効にしました。")
			return
		}
		for i := range m.Players {
			if m.Players[i].ID == s.id {
				m.Players[i].Ready = true
			}
		}
		if m.Players[0].Ready && m.Players[1].Ready {
			m.Phase = "countdown"
			m.Deadline = now.Add(3 * time.Second).UnixMilli()
		}
		l.matchState(m)
	default:
		fail("未対応の操作です。")
	}
}
func (l *lobby) tick(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, m := range l.matches {
		if m.Phase == "playing" {
			g := m.Game
			ended := false
			both := g.Actors[0].Disconnected && g.Actors[1].Disconnected
			if both {
				if now.Sub(g.Actors[0].disconnectedAt) >= 30*time.Second && now.Sub(g.Actors[1].disconnectedAt) >= 30*time.Second {
					l.finish(m, "双方の切断から30秒経過したため、無効試合となりました。")
					ended = true
				}
			} else {
				for _, a := range g.Actors {
					if a.Disconnected && now.Sub(a.disconnectedAt) >= 30*time.Second {
						winner := "相手"
						for _, other := range g.Actors {
							if other.ID != a.ID {
								winner = other.Name
							}
						}
						l.finish(m, winner+"の勝利（相手が30秒間切断）")
						ended = true
						break
					}
				}
			}
			if ended {
				continue
			}
			steps := 0
			for now.Sub(g.lastStep) >= 50*time.Millisecond && steps < 10 {
				g.advance()
				g.lastStep = g.lastStep.Add(50 * time.Millisecond)
				steps++
			}
			if steps > 0 && g.Step%2 == 0 {
				for _, s := range l.sessions {
					if s.matchID == m.ID {
						l.snapshot(s, m)
					}
				}
			}
			continue
		}
		if now.UnixMilli() < m.Deadline {
			continue
		}
		switch m.Phase {
		case "loading":
			l.finish(m, "準備が30秒以内に完了しなかったため、マッチを無効にしました。")
		case "countdown":
			m.Phase = "playing"
			m.Game = newGame(m, now)
			m.Deadline = 0
			l.matchState(m)
		}
	}
	for token, s := range l.sessions {
		if len(s.connections) == 0 && s.matchID == "" && now.Sub(s.seen) > 5*time.Minute {
			delete(l.sessions, token)
		}
	}
}
func (l *lobby) serveWS(w http.ResponseWriter, r *http.Request) {
	l.mu.Lock()
	cookie, err := r.Cookie("auxilia_moba_session")
	if err != nil || l.sessions[cookie.Value] == nil {
		l.mu.Unlock()
		http.Error(w, "session required", 401)
		return
	}
	s := l.sessions[cookie.Value]
	upgrader := websocket.Upgrader{CheckOrigin: l.originOK, HandshakeTimeout: 5 * time.Second}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		l.mu.Unlock()
		return
	}
	c := &client{conn: conn, send: make(chan []byte, 64), session: s}
	s.connections[c] = true
	if m := l.matches[s.matchID]; m != nil && m.Game != nil {
		for _, a := range m.Game.Actors {
			if a.ID == s.id {
				a.Disconnected = false
			}
		}
	}
	s.seen = time.Now()
	l.state(s, "")
	l.presence()
	l.mu.Unlock()
	defer func() {
		conn.Close()
		l.mu.Lock()
		defer l.mu.Unlock()
		delete(s.connections, c)
		close(c.send)
		s.seen = time.Now()
		if len(s.connections) == 0 {
			l.removeQueue(s)
			if m := l.matches[s.matchID]; m != nil {
				if m.Phase == "playing" {
					for _, a := range m.Game.Actors {
						if a.ID == s.id {
							a.Disconnected = true
							a.disconnectedAt = time.Now()
							a.stop()
						}
					}
				} else {
					l.finish(m, "相手との接続が切れたため、マッチを無効にしました。")
				}
			}
		}
		l.presence()
	}()
	go func() {
		ping := time.NewTicker(5 * time.Second)
		defer ping.Stop()
		defer conn.Close()
		for {
			select {
			case b, ok := <-c.send:
				if !ok {
					return
				}
				conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if conn.WriteMessage(websocket.TextMessage, b) != nil {
					return
				}
			case <-ping.C:
				if conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)) != nil {
					return
				}
			}
		}
	}()
	conn.SetReadLimit(4096)
	conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(15 * time.Second)) })
	window := time.Now()
	count := 0
	for {
		var cmd command
		if err := conn.ReadJSON(&cmd); err != nil {
			return
		}
		now := time.Now()
		if now.Sub(window) >= time.Second {
			window = now
			count = 0
		}
		count++
		if count > 60 {
			return
		}
		l.mu.Lock()
		l.handle(c, cmd, now)
		l.mu.Unlock()
	}
}
func main() {
	l := newLobby()
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for now := range ticker.C {
			l.tick(now)
		}
	}()
	address, err := listenAddress()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("League of Auxilia lobby listening on %s", address)
	server := &http.Server{Addr: address, Handler: l.handler(), ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.ListenAndServe())
}
