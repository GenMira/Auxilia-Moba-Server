package main

import (
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type event struct {
	Type    string `json:"type"`
	Phase   string `json:"phase"`
	Active  int    `json:"active"`
	SelfID  string `json:"selfId"`
	Match   *match `json:"match"`
	Message string `json:"message"`
}
type fixture struct {
	l      *lobby
	server *httptest.Server
}

func setup(t *testing.T) fixture {
	t.Helper()
	l := newLobby()
	server := httptest.NewServer(l.handler())
	t.Cleanup(server.Close)
	return fixture{l, server}
}
func (f fixture) connect(t *testing.T, cookie *http.Cookie) (*websocket.Conn, *http.Cookie) {
	t.Helper()
	if cookie == nil {
		response, err := http.Post(f.server.URL+"/api/session", "application/json", nil)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		cookie = response.Cookies()[0]
	}
	header := http.Header{}
	header.Set("Cookie", cookie.String())
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(f.server.URL, "http")+"/ws", header)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c, cookie
}
func await(t *testing.T, c *websocket.Conn, predicate func(event) bool) event {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		var e event
		if err := c.ReadJSON(&e); err != nil {
			t.Fatal(err)
		}
		if predicate(e) {
			return e
		}
	}
}
func phase(t *testing.T, c *websocket.Conn, p string) event {
	t.Helper()
	return await(t, c, func(e event) bool { return e.Type == "state" && e.Phase == p })
}
func send(t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	if err := c.WriteJSON(v); err != nil {
		t.Fatal(err)
	}
}
func join(t *testing.T, c *websocket.Conn) {
	send(t, c, command{Type: "queue", selection: selection{Name: "同じ名前", Character: "Sophie", Spells: []string{"flash", "ignite"}}})
}

func TestMatchLifecycleAndIsolation(t *testing.T) {
	f := setup(t)
	a, _ := f.connect(t, nil)
	b, _ := f.connect(t, nil)
	outsider, _ := f.connect(t, nil)
	phase(t, a, "entrance")
	phase(t, b, "entrance")
	phase(t, outsider, "entrance")
	join(t, a)
	phase(t, a, "queued")
	join(t, b)
	ea := phase(t, a, "loading")
	eb := phase(t, b, "loading")
	if ea.Match.ID != eb.Match.ID || ea.Match.Players[0].Team != "blue" || ea.Match.Players[1].Team != "red" || ea.Match.Players[0].ID == ea.Match.Players[1].ID {
		t.Fatal("invalid pairing")
	}
	send(t, outsider, command{Type: "ready", MatchID: ea.Match.ID})
	await(t, outsider, func(e event) bool { return e.Type == "error" })
	send(t, a, command{Type: "cancel"})
	await(t, a, func(e event) bool { return e.Type == "error" })
	phase(t, a, "loading")
	send(t, a, command{Type: "ready", MatchID: ea.Match.ID})
	phase(t, a, "loading")
	send(t, b, command{Type: "ready", MatchID: ea.Match.ID})
	countdown := phase(t, a, "countdown")
	phase(t, b, "countdown")
	f.l.tick(time.UnixMilli(countdown.Match.Deadline - 1))
	f.l.mu.Lock()
	if f.l.matches[ea.Match.ID].Phase != "countdown" {
		t.Error("countdown ended early")
	}
	f.l.mu.Unlock()
	f.l.tick(time.UnixMilli(countdown.Match.Deadline))
	phase(t, a, "playing")
	phase(t, b, "playing")
	send(t, a, command{Type: "leave", MatchID: ea.Match.ID})
	phase(t, a, "entrance")
	phase(t, b, "entrance")
	// No match state is broadcast to the third player; its independent queue still works.
	join(t, outsider)
	phase(t, outsider, "queued")
	f.l.mu.Lock()
	defer f.l.mu.Unlock()
	if len(f.l.matches) != 0 || len(f.l.queue) != 1 {
		t.Fatal("room cleanup failed")
	}
}
func TestDuplicateSessionAndCancelBeforeMatch(t *testing.T) {
	f := setup(t)
	a, cookie := f.connect(t, nil)
	duplicate, _ := f.connect(t, cookie)
	ea := phase(t, a, "entrance")
	ed := phase(t, duplicate, "entrance")
	if ea.SelfID != ed.SelfID {
		t.Fatal("session changed")
	}
	await(t, duplicate, func(e event) bool { return e.Type == "presence" && e.Active == 1 })
	join(t, a)
	phase(t, a, "queued")
	phase(t, duplicate, "queued")
	join(t, duplicate)
	phase(t, duplicate, "queued")
	send(t, a, command{Type: "cancel"})
	phase(t, a, "entrance")
	phase(t, duplicate, "entrance")
	b, _ := f.connect(t, nil)
	phase(t, b, "entrance")
	join(t, b)
	phase(t, b, "queued")
	f.l.mu.Lock()
	if len(f.l.matches) != 0 || len(f.l.queue) != 1 {
		t.Error("self matching or ghost queue")
	}
	f.l.mu.Unlock()
	join(t, a)
	phase(t, a, "loading")
	phase(t, b, "loading")
	duplicate.Close() // One tab closing must not invalidate an active session.
}
func TestValidationAndLoadTimeout(t *testing.T) {
	f := setup(t)
	a, _ := f.connect(t, nil)
	phase(t, a, "entrance")
	for _, s := range []selection{
		{Name: "", Character: "Sophie", Spells: []string{"flash", "ignite"}},
		{Name: strings.Repeat("字", 17), Character: "Sophie", Spells: []string{"flash", "ignite"}},
		{Name: "a\nb", Character: "Sophie", Spells: []string{"flash", "ignite"}},
		{Name: "valid", Character: "unknown", Spells: []string{"flash", "ignite"}},
		{Name: "valid", Character: "Chiyo", Spells: []string{"flash", "flash"}},
		{Name: "valid", Character: "Chiyo", Spells: []string{"flash", "unknown"}},
	} {
		send(t, a, command{Type: "queue", selection: s})
		await(t, a, func(e event) bool { return e.Type == "error" })
	}
	b, _ := f.connect(t, nil)
	phase(t, b, "entrance")
	join(t, a)
	join(t, b)
	e := phase(t, a, "loading")
	phase(t, b, "loading")
	f.l.tick(time.UnixMilli(e.Match.Deadline))
	phase(t, a, "entrance")
	phase(t, b, "entrance")
	join(t, a)
	phase(t, a, "queued")
	join(t, b)
	phase(t, a, "loading")
	phase(t, b, "loading")
	b.Close()
	phase(t, a, "entrance")
}
func TestHandshakeSecurity(t *testing.T) {
	f := setup(t)
	response, err := http.Get(f.server.URL + "/ws")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatal("unauthenticated socket accepted")
	}
	request, _ := http.NewRequest("POST", f.server.URL+"/api/session", nil)
	request.Header.Set("Origin", "https://evil.example")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("foreign origin accepted")
	}
}
