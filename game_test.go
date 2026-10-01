package main

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func testGame() (*lobby, *match, *client) {
	l := newLobby()
	m := &match{ID: "test", Phase: "playing", Players: [2]player{{ID: "a", Team: "blue", selection: selection{Name: "A", Character: "Sophie"}}, {ID: "b", Team: "red", selection: selection{Name: "B", Character: "Nadia"}}}}
	m.Game = newGame(m, time.Unix(0, 0))
	l.matches[m.ID] = m
	s := &session{id: "a", matchID: m.ID, connections: map[*client]bool{}}
	c := &client{session: s, send: make(chan []byte, 64)}
	s.connections[c] = true
	l.sessions["token"] = s
	return l, m, c
}
func advance(g *game, seconds float64) {
	for n := 0; n < int(math.Round(seconds/.05)); n++ {
		g.advance()
	}
}
func TestPathAndAuthoritativeMovement(t *testing.T) {
	for _, pair := range [][2]point{{{200, 0}, {2800, 0}}, {{5800, 0}, {3200, 0}}, {{200, 0}, {400, 0}}, {{200, 0}, {-900, -999}}, {{3000, 0}, {2700, 550}}} {
		route := findPath(pair[0], pair[1])
		if len(route) == 0 {
			t.Fatalf("no path: %v", pair)
		}
		start := pair[0]
		for _, p := range route {
			if !clearSegment(start, p) {
				t.Fatalf("path crosses obstacle: %v → %v", start, p)
			}
			start = p
		}
		if !legal(start) {
			t.Fatal("illegal destination")
		}
	}
	l, m, c := testGame()
	a := m.Game.Actors[0]
	target := point{200, 600}
	l.gameInput(c, command{Type: "move", MatchID: m.ID, Position: &target, Sequence: 1})
	advance(m.Game, 1)
	if math.Abs(distance(point{200, 0}, a.Position)-350) > .001 {
		t.Fatalf("movement speed: %v", a.Position)
	}
	l.gameInput(c, command{Type: "stop", MatchID: m.ID, Sequence: 1})
	if len(a.path) == 0 {
		t.Fatal("duplicate sequence accepted")
	}
	l.gameInput(c, command{Type: "stop", MatchID: m.ID, Sequence: 2})
	before := a.Position
	advance(m.Game, 1)
	if a.Position != before {
		t.Fatal("stop failed")
	}
	l.gameInput(c, command{Type: "move", MatchID: "foreign", Position: &target, Sequence: 3})
	if len(a.path) > 0 {
		t.Fatal("foreign match accepted")
	}
	if m.Game.Actors[1].Position != spawn("red") {
		t.Fatal("moved opponent")
	}
}
func TestVisibilityAndWirePrivacy(t *testing.T) {
	l, m, c := testGame()
	g := m.Game
	a, b := g.Actors[0], g.Actors[1]
	if g.visible(a, b) {
		t.Fatal("enemy spawn visible")
	}
	l.snapshot(c.session, m)
	var message struct{ Actors []avatar }
	json.Unmarshal(<-c.send, &message)
	if len(message.Actors) != 1 || message.Actors[0].ID != a.ID {
		t.Fatal("hidden position leaked")
	}
	a.Position = point{3000, 0}
	b.Position = point{2800, 550}
	if g.visible(a, b) {
		t.Fatal("bush leaked")
	}
	a.Position = point{2700, 550}
	if !g.visible(a, b) {
		t.Fatal("same bush invisible")
	}
	a.HP = 0
	if g.visible(a, b) {
		t.Fatal("dead character gives bush vision")
	}
	a.HP = 600
	a.Position = point{3000, 0}
	b.revealedUntil = 2
	if !g.visible(a, b) {
		t.Fatal("attack reveal missing")
	}
}
func TestAttackCancelDoesNotResetCadence(t *testing.T) {
	l, m, c := testGame()
	g := m.Game
	a, b := g.Actors[0], g.Actors[1]
	a.Position = point{3000, 0}
	b.Position = point{3100, 0}
	l.gameInput(c, command{Type: "attack", MatchID: m.ID, Target: b.ID, Sequence: 1})
	advance(g, .1)
	next := a.nextAttack
	l.gameInput(c, command{Type: "stop", MatchID: m.ID, Sequence: 2})
	l.gameInput(c, command{Type: "attack", MatchID: m.ID, Target: b.ID, Sequence: 3})
	advance(g, .5)
	if a.nextAttack != next || b.HP != b.Stats.HP {
		t.Fatal("cancel bypassed attack interval")
	}
	advance(g, 1.2)
	if b.HP >= b.Stats.HP {
		t.Fatal("ranged attack missed")
	}
}
func TestDeathRewardsRespawnAndRecall(t *testing.T) {
	l, m, c := testGame()
	g := m.Game
	a, b := g.Actors[0], g.Actors[1]
	a.Position = point{3000, 0}
	b.Position = point{3100, 0}
	b.HP = 10
	b.Kills = 3
	b.streak = 2
	l.gameInput(c, command{Type: "attack", MatchID: m.ID, Target: b.ID, Sequence: 1})
	advance(g, 1)
	if b.HP != 0 || a.Kills != 1 || a.Gold != 500 || b.Deaths != 1 || b.streak != 0 {
		t.Fatalf("death/reward mismatch: %+v / %+v", a, b)
	}
	death := b.RespawnAt
	if death < 15 || death > 16 {
		t.Fatalf("wrong respawn: %v", death)
	}
	advance(g, 16)
	if b.HP != b.Stats.HP || b.Mana != b.Stats.Mana || b.Position != spawn("red") {
		t.Fatal("respawn failed")
	}
	a.HP = 100
	l.gameInput(c, command{Type: "recall", MatchID: m.ID, Sequence: 2})
	advance(g, 7.95)
	if a.Position == spawn("blue") {
		t.Fatal("early recall")
	}
	advance(g, .1)
	if a.Position != spawn("blue") || a.HP > 120 {
		t.Fatal("recall teleported incorrectly or healed")
	}
}
func TestMeleeMutualKillsAndDamageInterruptsRecall(t *testing.T) {
	_, m, _ := testGame()
	g := m.Game
	a, b := g.Actors[0], g.Actors[1]
	a.Character = "Jude"
	a.Stats = characterDefinitions["Jude"]
	a.Position = point{3000, 0}
	b.Position = point{3100, 0}
	a.HP = 5
	b.HP = 5
	a.windupUntil = .05
	b.windupUntil = .05
	a.attackTarget = b.ID
	b.attackTarget = a.ID
	a.attackDamage = 50
	b.attackDamage = 55
	a.RecallUntil = 8
	advance(g, .05)
	if a.HP != 0 || b.HP != 0 || a.Kills != 1 || b.Kills != 1 || a.RespawnAt != 5.05 || b.RespawnAt != 5.05 || a.RecallUntil != 0 {
		t.Fatal("mutual kills/recall/death time incorrect")
	}
}

func TestRecallInterruptedOnCompletionTick(t *testing.T) {
	_, m, _ := testGame()
	g := m.Game
	a, b := g.Actors[0], g.Actors[1]
	a.Position = point{3000, 0}
	b.Position = point{3100, 0}
	a.RecallUntil = .05
	b.attackTarget = a.ID
	b.windupUntil = .05
	b.attackDamage = 55
	advance(g, .05)
	if a.Position != (point{3000, 0}) || a.RecallUntil != 0 || a.HP >= a.Stats.HP {
		t.Fatal("completion tick damage must interrupt recall")
	}
}
func TestProjectileSurvivesCasterDeath(t *testing.T) {
	_, m, _ := testGame()
	g := m.Game
	a, b := g.Actors[0], g.Actors[1]
	a.Position = point{3000, 0}
	b.Position = point{3450, 0}
	a.HP = 0
	a.RespawnAt = 5
	g.Projectiles = []projectile{{ID: 1, Owner: a.ID, Target: b.ID, Position: a.Position, damage: 50}}
	advance(g, .3)
	if b.HP >= b.Stats.HP {
		t.Fatal("caster death removed projectile")
	}
}
func TestPlayingReconnectAndGrace(t *testing.T) {
	f := setup(t)
	a, cookie := f.connect(t, nil)
	b, _ := f.connect(t, nil)
	phase(t, a, "entrance")
	phase(t, b, "entrance")
	join(t, a)
	join(t, b)
	e := phase(t, a, "loading")
	phase(t, b, "loading")
	send(t, a, command{Type: "ready", MatchID: e.Match.ID})
	phase(t, a, "loading")
	send(t, b, command{Type: "ready", MatchID: e.Match.ID})
	countdown := phase(t, a, "countdown")
	phase(t, b, "countdown")
	f.l.tick(time.UnixMilli(countdown.Match.Deadline))
	phase(t, a, "playing")
	phase(t, b, "playing")
	// Closing one session preserves the room; its next connection gets a full world.
	a.Close()
	deadline := time.Now().Add(time.Second)
	for {
		f.l.mu.Lock()
		m := f.l.matches[e.Match.ID]
		disconnected := false
		if m != nil {
			for _, v := range m.Game.Actors {
				if v.ID == e.SelfID {
					disconnected = v.Disconnected
				}
			}
		}
		f.l.mu.Unlock()
		if disconnected {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("disconnect not detected")
		}
		time.Sleep(time.Millisecond)
	}
	restored, _ := f.connect(t, cookie)
	state := phase(t, restored, "playing")
	if state.SelfID != e.SelfID || state.Match.ID != e.Match.ID {
		t.Fatal("session not restored")
	}
	await(t, restored, func(e event) bool { return e.Type == "world" })
	f.l.mu.Lock()
	m := f.l.matches[e.Match.ID]
	for _, v := range m.Game.Actors {
		if v.ID == e.SelfID && v.Disconnected {
			t.Error("still disconnected")
		}
	}
	f.l.mu.Unlock()
}
