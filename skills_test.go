package main

import (
	"encoding/json"
	"math"
	"testing"
)

func skillGame(character string) (*game, *avatar, *avatar) {
	_, m, _ := testGame()
	g := m.Game
	a, b := g.Actors[0], g.Actors[1]
	a.Character = character
	a.Stats = characterDefinitions[character]
	a.HP = a.Stats.HP
	a.Mana = a.Stats.Mana
	a.Position = point{3000, 0}
	b.Position = point{3150, 0}
	a.Stats.HPRegen = 0
	b.Stats.HPRegen = 0
	a.Stats.ManaRegen = 0
	b.Stats.ManaRegen = 0
	return g, a, b
}
func cast(t *testing.T, g *game, a *avatar, slot string, p point) {
	t.Helper()
	if reason := g.startCast(a, command{Slot: slot, Position: &p}); reason != "" {
		t.Fatal(reason)
	}
}
func near(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("got %.8f want %.8f", got, want)
	}
}

func TestSkillCostsTimingSnapshotAndRejection(t *testing.T) {
	g, a, b := skillGame("Sophie")
	cast(t, g, a, "q", point{3600, 0})
	near(t, a.Mana, 290)
	near(t, a.cooldowns[0], 6)
	a.Stats.Attack = 500 // In-flight cast damage must retain the original attack.
	if g.startCast(a, command{Slot: "w", Position: &b.Position}) == "" {
		t.Fatal("cast overlap accepted")
	}
	advance(g, .15)
	near(t, b.HP, b.Stats.HP)
	advance(g, .05)
	near(t, b.HP, b.Stats.HP-90)
	near(t, a.moveSpeed(.2), 380)
	if g.startCast(a, command{Slot: "q", Position: &b.Position}) != "クールダウン中" {
		t.Fatal("cooldown ignored")
	}
	a.Mana = 0
	if g.startCast(a, command{Slot: "w", Position: &b.Position}) != "マナ不足" {
		t.Fatal("mana ignored")
	}
	near(t, a.cooldowns[1], 0)
	a.Mana = 100
	if g.startCast(a, command{Slot: "w", Position: &point{math.NaN(), 0}}) == "" {
		t.Fatal("NaN accepted")
	}
	near(t, a.Mana, 100)
}
func TestSophieAreaAndInvisibleHit(t *testing.T) {
	g, a, b := skillGame("Sophie")
	b.Position = point{2800, 550}
	if g.visible(a, b) {
		t.Fatal("fixture must be hidden")
	}
	cast(t, g, a, "w", b.Position)
	advance(g, .4)
	near(t, b.HP, b.Stats.HP-70)
	near(t, b.moveSpeed(.4), 360)
	near(t, a.moveSpeed(.4), 380)
	near(t, b.Statuses[0].Until, 2.4)
	advance(g, 2)
	near(t, b.moveSpeed(2.4), 450)
	near(t, a.moveSpeed(2.4), 350)
}
func TestPointRangeClampAndMissConsumption(t *testing.T) {
	g, a, b := skillGame("Sophie")
	cast(t, g, a, "w", point{9000, 0})
	near(t, a.Cast.Destination.S, 3700)
	advance(g, .4)
	near(t, b.HP, b.Stats.HP)
	near(t, a.Mana, 270)
	near(t, a.cooldowns[1], 10)
}
func TestLineBlockedAndConeEdgeIntersection(t *testing.T) {
	g, a, b := skillGame("Sophie")
	a.Position = point{3500, 0}
	b.Position = point{3990, 0}
	cast(t, g, a, "q", point{4200, 0})
	advance(g, .2)
	near(t, b.HP, b.Stats.HP)
	for _, tc := range []struct {
		p    point
		want bool
	}{{point{190, 190}, true}, {point{200, 160}, true}, {point{200, 240}, false}, {point{-34, 0}, true}, {point{-36, 0}, false}, {point{285, 0}, true}, {point{285.1, 0}, false}} {
		if got := coneIntersects(point{}, point{1, 0}, 250, 90, tc.p, 35); got != tc.want {
			t.Fatalf("cone edge %v: %v", tc.p, got)
		}
	}
}
func TestJudeSkillsCleanseAndReduction(t *testing.T) {
	g, a, b := skillGame("Jude")
	cast(t, g, a, "q", b.Position)
	advance(g, .2)
	near(t, b.HP, b.Stats.HP-90)
	near(t, b.moveSpeed(.2), 337.5)
	cast(t, g, a, "w", a.Position)
	advance(g, .3)
	near(t, b.HP, b.Stats.HP-210)
	a.HP = 100
	a.addStatus(statusEffect{ID: "poison", Source: b.ID, Kind: "poison", Until: 5, nextTick: 1, interval: 1, damage: 8})
	a.addStatus(statusEffect{ID: "slow", Source: b.ID, Kind: "slow", Until: 5, Value: .25})
	a.addStatus(statusEffect{ID: "ignite", Source: b.ID, Kind: "ignite", Until: 5, nextTick: 1, interval: 1, damage: 10, trueDamage: true})
	cast(t, g, a, "e", a.Position)
	advance(g, .3)
	near(t, a.HP, 170)
	if a.hasStatus("poison", .8) || a.hasStatus("slow", .8) || !a.hasStatus("ignite", .8) {
		t.Fatal("cleanse removed wrong effects")
	}
	advance(g, .2)
	near(t, a.HP, 160)
	for _, tc := range []struct {
		source       string
		truth        bool
		damage, want float64
	}{{"attack", false, 50, 40}, {"skill", false, 5, 0}, {"poison", false, 8, 8}, {"ignite", true, 10, 10}} {
		h := g.newHit(b, a, tc.damage, tc.source, 1)
		h.trueDamage = tc.truth
		got := g.applyDamage(h)
		near(t, got.hpDamage, tc.want)
	}
}
func TestProjectileSweepCollisionAndCasterDeath(t *testing.T) {
	for _, tc := range []struct {
		name           string
		origin, target point
		damage         bool
	}{{"hit", point{3000, 0}, point{3500, 0}, true}, {"blocked", point{3500, 0}, point{4000, 0}, false}, {"miss", point{3000, 0}, point{3500, 80}, false}} {
		t.Run(tc.name, func(t *testing.T) {
			g, a, b := skillGame("Sophie")
			a.Position = tc.origin
			b.Position = tc.target
			cast(t, g, a, "e", point{4500, 0})
			advance(g, .6)
			if len(g.Projectiles) != 1 {
				t.Fatal("no projectile")
			}
			a.HP = 0
			a.RespawnAt = 5
			advance(g, .65)
			want := b.Stats.HP
			if tc.damage {
				want -= 170
			}
			near(t, b.HP, want)
			if len(g.Projectiles) != 0 {
				t.Fatal("projectile failed to expire")
			}
		})
	}
	// The first contact in a 50ms step is detected even if the center ends past it.
	g, a, b := skillGame("Sophie")
	b.Position = point{3140, 0}
	g.Projectiles = []projectile{{ID: 1, Owner: a.ID, Kind: "skill", Position: point{3050, 0}, Direction: point{1, 0}, Width: 70, Remaining: 900, damage: 170}}
	advance(g, .05)
	near(t, b.HP, b.Stats.HP-170)
}
func TestInterruptionAndCooldownThroughDeath(t *testing.T) {
	for _, kind := range []string{"stun", "silence", "root"} {
		t.Run(kind, func(t *testing.T) {
			g, a, b := skillGame("Sophie")
			cast(t, g, a, "q", b.Position)
			a.addStatus(statusEffect{ID: kind, Kind: kind, Until: 1})
			advance(g, .2)
			want := b.Stats.HP
			if kind == "root" {
				want -= 90
			}
			near(t, b.HP, want)
			near(t, a.Mana, 290)
			near(t, a.cooldowns[0], 6)
		})
	}
	g, a, b := skillGame("Sophie")
	a.HP = 1
	cast(t, g, a, "e", b.Position)
	b.attackTarget = a.ID
	b.windupUntil = .05
	b.attackDamage = 100
	advance(g, .05)
	if a.Cast != nil || len(a.Statuses) != 0 {
		t.Fatal("death left cast/statuses")
	}
	near(t, a.cooldowns[2], 18)
	advance(g, 5)
	if a.HP <= 0 {
		t.Fatal("no respawn")
	}
	near(t, a.cooldowns[2], 18)
}
func TestHealingBeforeDamageAndMutualSkills(t *testing.T) {
	g, a, b := skillGame("Jude")
	a.HP = 20
	cast(t, g, a, "e", a.Position)
	b.attackTarget = a.ID
	b.windupUntil = .3
	b.attackDamage = 70
	advance(g, .3)
	near(t, a.HP, 30)
	g, a, b = skillGame("Jude")
	b.Character = "Jude"
	b.Stats = characterDefinitions["Jude"]
	b.Stats.HPRegen = 0
	a.HP = 50
	b.HP = 50
	b.Mana = 200
	cast(t, g, a, "w", a.Position)
	cast(t, g, b, "w", b.Position)
	advance(g, .3)
	if a.HP != 0 || b.HP != 0 || a.Kills != 1 || b.Kills != 1 {
		t.Fatal("skills must allow mutual kill")
	}
}
func TestSlowStackingZeroDamageProcAndShieldOrder(t *testing.T) {
	g, a, b := skillGame("Sophie")
	b.Character = "Jude"
	b.RecallUntil = 8
	h := g.newHit(a, b, 5, "skill", 0)
	h.slow = .2
	h.slowDuration = 2
	h.effectID = "bloom"
	g.applyDamage(h)
	near(t, b.HP, b.Stats.HP)
	near(t, b.RecallUntil, 8)
	near(t, a.moveSpeed(0), 380)
	b.addStatus(statusEffect{ID: "other", Source: a.ID, Kind: "slow", Until: 5, Value: .25})
	near(t, b.moveSpeed(0), 337.5)
	b.addStatus(statusEffect{ID: "first", Kind: "shield", Until: 5, Value: 20})
	b.addStatus(statusEffect{ID: "second", Kind: "shield", Until: 5, Value: 100})
	result := g.applyDamage(g.newHit(a, b, 60, "attack", 0))
	near(t, result.mitigated, 50)
	near(t, result.absorbed, 50)
	near(t, result.hpDamage, 0)
	near(t, b.RecallUntil, 0)
	near(t, b.Statuses[len(b.Statuses)-2].Value, 0)
	near(t, b.Statuses[len(b.Statuses)-1].Value, 70)
}
func TestCastInputOwnershipRejectionAndWirePrivacy(t *testing.T) {
	l, m, c := testGame()
	a, b := m.Game.Actors[0], m.Game.Actors[1]
	l.gameInput(c, command{Type: "cast", MatchID: "foreign", Slot: "q", Sequence: 1, Position: &b.Position})
	if a.Cast != nil {
		t.Fatal("foreign match accepted")
	}
	<-c.send
	l.gameInput(c, command{Type: "cast", MatchID: m.ID, Slot: "q", Sequence: 2, Position: &b.Position})
	<-c.send
	l.gameInput(c, command{Type: "move", MatchID: m.ID, Sequence: 3, Position: &b.Position})
	var result struct {
		OK     bool
		Reason string
	}
	json.Unmarshal(<-c.send, &result)
	if result.OK || result.Reason == "" || len(a.path) > 0 || b.Cast != nil {
		t.Fatal("cast lock/ownership failed")
	}
	l.snapshot(c.session, m)
	var wire struct{ Actors []avatar }
	json.Unmarshal(<-c.send, &wire)
	if len(wire.Actors) != 1 || len(wire.Actors[0].Skills) != 3 || wire.Actors[0].Cast == nil {
		t.Fatal("snapshot missing skills or leaked hidden enemy")
	}
}

func TestTargetAimValidationAndRecheck(t *testing.T) {
	// Exercise the common target mode without enabling a step-3 character.
	original := skillDefinitions["Sophie"]
	t.Cleanup(func() { skillDefinitions["Sophie"] = original })
	defs := original
	defs[0].Aim, defs[0].Shape, defs[0].Range = "target", "target", 200
	skillDefinitions["Sophie"] = defs
	g, a, b := skillGame("Sophie")
	b.Position = point{3300, 0}
	cmd := command{Slot: "q", Target: b.ID}
	if g.startCast(a, cmd) != "射程外" {
		t.Fatal("target outside range accepted")
	}
	near(t, a.Mana, 330)
	near(t, a.cooldowns[0], 0)
	b.Position = point{3150, 0}
	if reason := g.startCast(a, cmd); reason != "" {
		t.Fatal(reason)
	}
	b.Position = point{3300, 0}
	advance(g, .2)
	near(t, b.HP, b.Stats.HP)
	near(t, a.Mana, 290)
}

func TestStatusRefreshPreservesTicksAndFinalTick(t *testing.T) {
	g, a, b := skillGame("Jude")
	s := statusEffect{ID: "poison", Source: b.ID, Kind: "poison", Until: 3, nextTick: 1, interval: 1, damage: 8}
	a.addStatus(s)
	advance(g, .5)
	s.Until = 3.5
	s.nextTick = 1.5
	a.addStatus(s)
	advance(g, .5)
	near(t, a.HP, a.Stats.HP-8)
	advance(g, 2.5)
	near(t, a.HP, a.Stats.HP-24)
	if len(a.Statuses) != 0 {
		t.Fatal("expired poison retained")
	}
	a.addStatus(statusEffect{ID: "final", Source: b.ID, Kind: "poison", Until: 4, nextTick: 4, interval: 1, damage: 8})
	advance(g, .5)
	near(t, a.HP, a.Stats.HP-32)
}

func TestProjectileBoundaryAndRankValues(t *testing.T) {
	g, a, b := skillGame("Sophie")
	a.Position = point{3000, 650}
	b.Position = point{3300, 0}
	cast(t, g, a, "e", point{3000, 1000})
	advance(g, 1)
	if len(g.Projectiles) != 0 {
		t.Fatal("projectile crossed map boundary")
	}
	for _, character := range []string{"Sophie", "Jude"} {
		g, a, _ := skillGame(character)
		for rank := 1; rank <= 3; rank++ {
			a.ranks = [3]int{rank, rank, rank}
			for i, v := range g.skillViews(a) {
				d := skillDefinitions[character][i]
				near(t, v.Amount, d.base[rank-1]+d.attackRatio*50)
			}
		}
	}
}
