package main

import (
	"math"
	"testing"
)

func TestNadiaBlinkTimingAndTargetRecheck(t *testing.T) {
	for _, dist := range []float64{100, 480} {
		t.Run(string(rune(int(dist))), func(t *testing.T) {
			g, a, b := skillGame("Nadia")
			b.Position = point{3000 + dist, 0}
			if reason := g.startCast(a, command{Slot: "e", Target: b.ID}); reason != "" {
				t.Fatal(reason)
			}
			advance(g, .15)
			near(t, a.Position.S, 3000)
			advance(g, .05)
			near(t, a.Position.S, 3000+math.Min(300, math.Max(0, dist-150)))
			near(t, b.HP, b.Stats.HP)
			advance(g, .3)
			near(t, b.HP, b.Stats.HP-149.5)
			if !b.hasStatus("poison", .5) {
				t.Fatal("no poison")
			}
		})
	}
	g, a, b := skillGame("Nadia")
	b.Position = point{3480.01, 0}
	if g.startCast(a, command{Slot: "e", Target: b.ID}) != "射程外" {
		t.Fatal("outside range accepted")
	}
	near(t, a.Mana, 300)
	b.Position = point{3480, 0}
	if r := g.startCast(a, command{Slot: "e", Target: b.ID}); r != "" {
		t.Fatal(r)
	}
	b.Position = point{3500, 0}
	advance(g, .5)
	near(t, a.Position.S, 3300)
	near(t, b.HP, b.Stats.HP)
	near(t, a.Mana, 240)
}
func TestBlinkRootBeforeAndAfter(t *testing.T) {
	for _, character := range []string{"Nadia", "Chiyo"} {
		for _, before := range []bool{true, false} {
			g, a, b := skillGame(character)
			b.Position = point{3400, 0}
			slot := "w"
			delay := .1
			if character == "Nadia" {
				slot = "e"
				delay = .2
			}
			if reason := g.startCast(a, command{Slot: slot, Target: b.ID, Position: &b.Position}); reason != "" {
				t.Fatal(reason)
			}
			if !before {
				advance(g, delay)
			}
			a.addStatus(statusEffect{ID: "root", Kind: "root", Until: 2})
			advance(g, .6)
			if before {
				near(t, a.Position.S, 3000)
				near(t, b.HP, b.Stats.HP)
			} else if a.Position.S == 3000 {
				t.Fatal("root undid blink")
			}
			if a.Cast != nil {
				t.Fatal("cast did not end")
			}
			if a.cooldowns[slotIndex(slot)] == 0 {
				t.Fatal("refunded cooldown")
			}
		}
	}
}
func TestChiyoSkillsFrozenBonusAndLandingOrigin(t *testing.T) {
	g, a, b := skillGame("Chiyo")
	a.HP = a.Stats.HP * .6
	cast(t, g, a, "q", b.Position)
	a.HP = 1
	advance(g, .2)
	near(t, b.HP, b.Stats.HP-118.8)
	g, a, b = skillGame("Chiyo")
	b.Position = point{3290, 0}
	cast(t, g, a, "w", b.Position)
	advance(g, .05)
	near(t, a.Position.S, 3000)
	advance(g, .05)
	near(t, a.Position.S, 3100)
	advance(g, .3)
	near(t, b.HP, b.Stats.HP-140.8)
	g, a, b = skillGame("Chiyo")
	if reason := g.startCast(a, command{Slot: "e", Target: b.ID}); reason != "" {
		t.Fatal(reason)
	}
	advance(g, .3)
	near(t, b.HP, b.Stats.HP-166)
}
func TestPassivesAndPoisonRefresh(t *testing.T) {
	g, a, b := skillGame("Nadia")
	b.Character = "Jude"
	for i, want := range []float64{45, 45, 72.5} {
		result := g.applyDamage(g.newHit(a, b, 55, "attack", 0))
		near(t, result.hpDamage, want)
		if a.AttackCount != (i+1)%3 {
			t.Fatal("count")
		}
	}
	cast(t, g, a, "q", a.Position)
	advance(g, .2)
	near(t, b.Statuses[0].nextTick, 1.2)
	cast(t, g, a, "w", b.Position)
	advance(g, .3)
	near(t, b.Statuses[0].nextTick, 1.2)
	near(t, b.Statuses[0].Until, 3.5)
	hp := b.HP
	advance(g, .7)
	near(t, b.HP, hp-8)
	g, a, b = skillGame("Chiyo")
	a.attackTarget = b.ID
	a.HP = a.Stats.HP * .6
	advance(g, .05)
	a.HP = 1
	advance(g, .35)
	near(t, b.HP, b.Stats.HP-66)
}
func TestFlashLandingAndValidation(t *testing.T) {
	for _, tc := range []struct {
		start, end point
		expected   float64
	}{{point{3500, 0}, point{3900, 0}, 3689.989998}, {point{3550, 0}, point{3950, 0}, 3950}, {point{5900, 0}, point{6300, 0}, 5965}, {point{3000, 650}, point{3000, 900}, 3000}} {
		landed := blinkDestination(tc.start, tc.end)
		if !legal(landed) {
			t.Fatalf("illegal landing %+v", landed)
		}
		if math.Abs(landed.S-tc.expected) > .001 {
			t.Fatalf("landing %+v", landed)
		}
	}
	g, a, _ := skillGame("Nadia")
	a.spellIDs = [2]string{"flash", "ignite"}
	if g.castSpell(a, command{Slot: "d", Position: &a.Position}) != "移動距離不足" {
		t.Fatal("zero flash accepted")
	}
	near(t, a.spellCD[0], 0)
	a.Position = point{5965, 0}
	end := point{6500, 0}
	if g.castSpell(a, command{Slot: "d", Position: &end}) != "移動距離不足" {
		t.Fatal("blocked flash consumed")
	}
	near(t, a.spellCD[0], 0)
	a.Position = point{3000, 0}
	end = point{3400, 0}
	a.addStatus(statusEffect{ID: "root", Kind: "root", Until: 2})
	if g.castSpell(a, command{Slot: "d", Position: &end}) != "ルート中" {
		t.Fatal("root ignored")
	}
	a.cleanse("root")
	a.addStatus(statusEffect{ID: "silence", Kind: "silence", Until: 2})
	if r := g.castSpell(a, command{Slot: "d", Position: &end}); r != "" {
		t.Fatal(r)
	}
	near(t, a.Position.S, 3400)
	near(t, a.spellCD[0], 120)
	near(t, a.Mana, 300)
}
func TestIgniteBarrierAndDeathPersistence(t *testing.T) {
	g, a, b := skillGame("Nadia")
	a.spellIDs = [2]string{"ignite", "flash"}
	b.spellIDs = [2]string{"barrier", "flash"}
	b.Character = "Jude"
	if r := g.castSpell(b, command{Slot: "d"}); r != "" {
		t.Fatal(r)
	}
	if r := g.castSpell(a, command{Slot: "d", Target: b.ID}); r != "" {
		t.Fatal(r)
	}
	a.HP = 0
	a.RespawnAt = 20
	b.Position = spawn("red")
	advance(g, 4)
	near(t, b.HP, b.Stats.HP)
	near(t, b.Statuses[0].Value, 60)
	// Shield is inactive at exactly its expiration; final ignite tick still happens.
	advance(g, 1)
	near(t, b.HP, b.Stats.HP-10)
	if len(b.Statuses) != 0 {
		t.Fatal("expired effects retained")
	}
	near(t, a.spellCD[0], 70)
	near(t, b.spellCD[0], 90)
	g, a, b = skillGame("Nadia")
	b.spellIDs = [2]string{"barrier", "flash"}
	g.castSpell(b, command{Slot: "d"})
	h := g.newHit(a, b, 150, "ignite", 0)
	h.trueDamage = true
	r := g.applyDamage(h)
	near(t, r.absorbed, 100)
	near(t, r.hpDamage, 50)
	if b.hasStatus("shield", 0) {
		t.Fatal("empty shield retained")
	}
}
func TestGrowthUpgradeAndInFlightRank(t *testing.T) {
	g, a, b := skillGame("Nadia")
	a.XP = 300
	g.levelUp(a)
	if a.Level != 3 || a.SkillPoints != 2 {
		t.Fatal("growth")
	}
	near(t, a.Stats.HP, 810)
	cast(t, g, a, "q", a.Position)
	if r := g.upgrade(a, "q"); r != "" {
		t.Fatal(r)
	}
	advance(g, .2)
	near(t, b.HP, b.Stats.HP-(50+65*.5))
	if a.ranks[0] != 2 || a.SkillPoints != 1 {
		t.Fatal("upgrade")
	}
	if g.upgrade(a, "q") != "必要レベル未到達" {
		t.Fatal("rank 3 gated")
	}
	a.XP = 700
	g.levelUp(a)
	if a.Level != 5 {
		t.Fatal("level5")
	}
	if r := g.upgrade(a, "q"); r != "" {
		t.Fatal(r)
	}
	if g.upgrade(a, "q") != "最大ランク" {
		t.Fatal("rank cap")
	}
	a.HP = 0
	a.XP = 1100
	g.levelUp(a)
	if a.Level != 7 || a.XP != 0 || a.HP != 0 {
		t.Fatal("dead level-up/cap")
	}
	if g.upgrade(a, "w") != "死亡中" {
		t.Fatal("dead upgrade")
	}
	a.XP = 100
	g.levelUp(a)
	if a.XP != 0 {
		t.Fatal("max xp")
	}
}
func TestUpgradeSequenceAndSpellOwnership(t *testing.T) {
	l, m, c := testGame()
	a := m.Game.Actors[0]
	a.Level = 3
	a.SkillPoints = 2
	cmd := command{Type: "upgrade", Slot: "q", MatchID: m.ID, Sequence: 1}
	l.gameInput(c, cmd)
	l.gameInput(c, cmd)
	if a.ranks[0] != 2 || a.SkillPoints != 1 {
		t.Fatal("duplicate upgrade")
	}
	a.spellIDs = [2]string{"flash", "barrier"}
	dest := point{200, 400}
	l.gameInput(c, command{Type: "spell", Slot: "d", MatchID: "foreign", Sequence: 2, Position: &dest})
	near(t, a.spellCD[0], 0)
	l.gameInput(c, command{Type: "spell", Slot: "e", MatchID: m.ID, Sequence: 3, Position: &dest})
	near(t, a.spellCD[0], 0)
}
