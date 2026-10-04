package main

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func laneGame() *game { _, m, _ := testGame(); m.Game.disableNPC = false; return m.Game }
func minion(g *game, id, team string, p point) *avatar {
	a := &avatar{ID: id, Kind: "minion", Team: team, Position: p, anchor: p, HP: 100, Stats: characterDefinition{HP: 100, Attack: 20, Range: 150, AttackSpeed: 1, Speed: 300}, Statuses: []statusEffect{}}
	g.Minions = append(g.Minions, a)
	return a
}
func TestWaveScheduleAndNavigation(t *testing.T) {
	g := laneGame()
	advance(g, 9.95)
	if len(g.Minions) != 0 {
		t.Fatal("early spawn")
	}
	g.advance()
	if len(g.Minions) != 20 {
		t.Fatalf("wave: %d", len(g.Minions))
	}
	if g.Minions[0].Position != (point{600, -60}) || g.Minions[9].Position != (point{840, 60}) || g.Minions[10].Position != (point{5400, -60}) {
		t.Fatal("spawn formation changed")
	}
	for _, a := range g.Minions {
		if a.HP != 100 || !g.navigation(a).legal(a.Position) {
			t.Fatal("invalid spawn", a)
		}
	}
	advance(g, 5)
	for _, a := range g.Minions {
		if !g.navigation(a).legal(a.Position) {
			t.Fatal("crossed living structure", a.ID, a.Position)
		}
	}
	g.Step = 799
	g.Minions = nil
	g.advance()
	if len(g.Minions) != 20 {
		t.Fatal("second wave missing")
	}
	if g.Minions[0].ID != "minion-blue-000001-00" {
		t.Fatal("unstable wave ID")
	}
}
func TestMinionPriorityRetentionGuardAndWindup(t *testing.T) {
	g := laneGame()
	a, b := g.Actors[0], g.Actors[1]
	a.Position = point{3000, 0}
	b.Position = point{3120, 0}
	own := minion(g, "own", "blue", point{3000, 60})
	enemy := minion(g, "enemy", "red", point{3100, 60})
	if g.acquire(own) != enemy {
		t.Fatal("must prefer minion")
	}
	g.selectTarget(own, b)
	g.advanceNPC()
	if own.attackTarget != b.ID {
		t.Fatal("valid target not retained")
	}
	g.selectTarget(own, enemy)
	a.addStatus(statusEffect{ID: "shield", Kind: "shield", Until: 10, Value: 100})
	g.applyDamage(g.newHit(b, a, 20, "poison", 0))
	if own.attackTarget != b.ID || own.guardUntil != 3 {
		t.Fatal("shield damage did not trigger guard")
	}
	g.Step = 60
	g.advanceNPC()
	if own.attackTarget != enemy.ID {
		t.Fatal("guard did not expire")
	}
	g.disableNPC = true
	g.selectTarget(a, nil)
	g.selectTarget(b, nil)
	own.Position = point{3000, 60}
	enemy.Position = point{3100, 60}
	own.windupUntil = 3.3
	own.attackDamage = 20
	own.attackTarget = enemy.ID
	g.disableNPC = false
	g.Step = 65
	g.combat()
	if enemy.HP != 100 {
		t.Fatal("early melee impact")
	}
	g.Step = 66
	g.combat()
	if enemy.HP != 80 {
		t.Fatal("missing melee impact", enemy.HP)
	}
}
func TestTowerFireEscalationRetaliationAndPersistence(t *testing.T) {
	g := laneGame()
	tower := g.actor("blue-outer")
	enemy := g.Actors[1]
	ally := g.Actors[0]
	enemy.Position = point{2600, 0}
	enemy.HP = 5000
	enemy.Stats.HP = 5000
	ally.Position = point{2450, 0}
	m := minion(g, "target", "red", point{2400, 60})
	if g.acquire(tower) != m {
		t.Fatal("tower must prefer minion")
	}
	g.selectTarget(tower, m)
	tower.nextAttack = .8
	g.applyDamage(g.newHit(enemy, ally, 1, "poison", 0))
	if tower.attackTarget != enemy.ID || tower.nextAttack != .8 {
		t.Fatal("retaliation reset interval")
	}
	tower.nextAttack = 0
	g.combat()
	g.Step = 6
	g.combat()
	if len(g.Projectiles) != 1 || g.Projectiles[0].damage != 100 {
		t.Fatal("first tower shot", g.Projectiles)
	}
	g.Projectiles = nil
	g.Step = 20
	g.combat()
	g.Step = 26
	g.combat()
	if len(g.Projectiles) != 1 || g.Projectiles[0].damage != 200 {
		t.Fatal("second tower shot", g.Projectiles)
	}
	tower.HP = 0
	enemy.Position = point{3400, 0}
	m.HP = 0
	for i := 0; i < 20; i++ {
		g.Step++
		g.combat()
	}
	if enemy.HP != 4800 {
		t.Fatal("destroyed tower projectile must persist", enemy.HP)
	}
	tower.HP = 2000
	g.advanceNPC()
	if tower.shots != 0 {
		t.Fatal("invalid target did not reset damage")
	}
}
func TestStructureImmunitySupportUnlockAndCollision(t *testing.T) {
	g := laneGame()
	a := g.Actors[0]
	outer := g.actor("red-outer")
	inner := g.actor("red-inner")
	g.applyDamage(g.newHit(a, outer, 100, "skill", 0))
	g.applyDamage(g.newHit(a, inner, 100, "attack", 0))
	if outer.HP != 2000 || inner.HP != 2000 {
		t.Fatal("immunity failed")
	}
	a.Character = "Nadia"
	a.AttackCount = 2
	g.applyDamage(g.newHit(a, outer, 100, "attack", 0))
	if outer.HP != 1980 || a.AttackCount != 2 {
		t.Fatal("unsupported reduction/passive immunity failed")
	}
	support := minion(g, "support", "blue", point{2900, 0})
	g.applyDamage(g.newHit(a, outer, 100, "attack", 0))
	if outer.HP != 1880 {
		t.Fatal("support range boundary")
	}
	support.HP = 0
	g.applyDamage(g.newHit(a, outer, 100, "attack", 0))
	if outer.HP != 1860 {
		t.Fatal("dead minion grants support")
	}
	outer.HP = 0
	g.applyDamage(g.newHit(a, inner, 100, "attack", 0))
	if inner.HP != 2000 {
		t.Fatal("unlocked within same update")
	}
	g.unlockStructures()
	g.applyDamage(g.newHit(a, inner, 100, "attack", 0))
	if inner.HP != 1980 {
		t.Fatal("not unlocked next update")
	}
	if !g.navigation(a).legal(outer.Position) || !legal(point{3000, 0}) {
		t.Fatal("destroyed building collision")
	}
	if g.navigation(a).blinkDestination(point{3600, 0}, outer.Position) != outer.Position {
		t.Fatal("blink blocked by destroyed building")
	}
	outer.addStatus(statusEffect{Kind: "poison"})
	if len(outer.Statuses) != 0 {
		t.Fatal("structure accepted status")
	}
	if laneGame().actor("red-outer").HP != 2000 {
		t.Fatal("HP leaked across matches")
	}
}
func TestMinionRewardsAndNPCKillCredit(t *testing.T) {
	for _, tc := range []struct {
		name              string
		near, alive, last bool
		gold, xp, cs      int
	}{
		{"nearby", true, true, false, 5, 20, 0}, {"last hit", true, true, true, 15, 20, 1}, {"remote last hit", false, true, true, 15, 0, 1}, {"dead last hit", true, false, true, 15, 0, 1}, {"dead bystander", true, false, false, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := laneGame()
			a := g.Actors[0]
			a.Position = point{3000, 0}
			if !tc.near {
				a.Position = point{100, 0}
			}
			if !tc.alive {
				a.HP = 0
			}
			victim := minion(g, "dead", "red", point{3000, 0})
			victim.HP = 0
			var killer *avatar
			if tc.last {
				killer = a
			}
			g.rewardNPCDeaths(map[string]*avatar{victim.ID: killer})
			if a.Gold != 300+tc.gold || a.XP != tc.xp || a.LastHits != tc.cs {
				t.Fatal("wrong rewards", a.Gold, a.XP, a.LastHits)
			}
		})
	}
	for _, age := range []float64{10, 10.05} {
		t.Run(fmtAge(age), func(t *testing.T) {
			g := laneGame()
			a, b := g.Actors[0], g.Actors[1]
			a.Position = point{3000, 0}
			b.Position = point{3100, 0}
			g.applyDamage(g.newHit(a, b, 1, "skill", 0))
			g.Step = int64(math.Round(age / .05))
			b.HP = 10
			attacker := minion(g, "killer", "blue", point{3050, 0})
			attacker.attackTarget = b.ID
			attacker.windupUntil = age
			attacker.attackDamage = 20
			g.combat()
			want := 0
			if age == 10 {
				want = 1
			}
			if a.Kills != want || b.Deaths != 1 {
				t.Fatal("NPC kill credit", a.Kills, b.Deaths)
			}
		})
	}
}
func fmtAge(age float64) string {
	if age == 10 {
		return "credit at ten seconds"
	}
	return "expired credit"
}
func TestMinionSkillsAndCharacterOnlyTargets(t *testing.T) {
	g := laneGame()
	a := g.Actors[0]
	a.Character = "Nadia"
	a.Position = point{3000, 0}
	m := minion(g, "victim", "red", point{3100, 0})
	if reason := g.startCast(a, command{Slot: "q"}); reason != "" {
		t.Fatal(reason)
	}
	g.Step = 4
	g.combat()
	if m.HP >= 100 || !m.hasStatus("poison", .2) {
		t.Fatal("AOE did not affect minion")
	}
	a.Character = "Chiyo"
	a.Cast = nil
	if g.startCast(a, command{Slot: "e", Target: m.ID}) == "" {
		t.Fatal("Chiyo E accepted minion")
	}
	a.spellIDs[0] = "ignite"
	if g.castSpell(a, command{Slot: "d", Target: m.ID}) == "" {
		t.Fatal("ignite accepted minion")
	}
}
func TestVisionLastSeenAndDeadSources(t *testing.T) {
	g := laneGame()
	a, b := g.Actors[0], g.Actors[1]
	b.Position = point{4000, 0}
	if g.visible(a, b) {
		t.Fatal("enemy visible before shared source")
	}
	m := minion(g, "scout", "blue", point{3400, 0})
	if !g.visible(a, b) {
		t.Fatal("minion vision missing")
	}
	outer := g.actor("red-outer")
	outer.HP = 1234
	g.mapView(a)
	m.HP = 0
	outer.HP = 999
	payload, _ := json.Marshal(g.mapView(a))
	var view struct {
		Structures []structureView `json:"structures"`
	}
	json.Unmarshal(payload, &view)
	for _, s := range view.Structures {
		if s.ID == outer.ID && (s.HP != 1234 || s.Visible) {
			t.Fatal("hidden HP leaked", s)
		}
	}
	if g.visible(a, b) {
		t.Fatal("dead source grants vision")
	}
	b.Position = point{2700, 550}
	m.HP = 100
	m.Position = point{2700, 400}
	if g.visible(a, b) {
		t.Fatal("bush leak")
	}
	m.Position.T = 500
	if !g.visible(a, b) {
		t.Fatal("same bush should be visible")
	}
	g.actor("blue-outer").HP = 0
	m.HP = 0
	b.Position = point{3000, 0}
	if g.visible(a, b) {
		t.Fatal("destroyed tower grants vision")
	}
}
func TestSimultaneousBasesFreezeAndFinishedReconnect(t *testing.T) {
	l, m, c := testGame()
	g := m.Game
	for _, o := range g.Structures {
		if o.Kind == "tower" {
			o.HP = 0
		}
	}
	g.unlockStructures()
	for i, a := range g.Actors {
		target := g.actor("red-base")
		if i == 1 {
			target = g.actor("blue-base")
		}
		a.Character = "Chiyo"
		a.Position = point{target.Position.S - 150, 0}
		a.attackTarget = target.ID
		a.windupUntil = .05
		a.attackDamage = 10000
	}
	g.advance()
	if g.Winner != "draw" {
		t.Fatal("expected simultaneous draw", g.Winner)
	}
	step := g.Step
	gold := g.Actors[0].Gold
	advance(g, 10)
	if step != g.Step || gold != g.Actors[0].Gold {
		t.Fatal("end state not frozen")
	}
	l.tick(time.Unix(1, 0))
	if m.Phase != "finished" {
		t.Fatal("missing finished phase")
	}
	l.state(c.session, "")
	var world map[string]any
	for len(c.send) > 0 {
		var msg map[string]any
		json.Unmarshal(<-c.send, &msg)
		if msg["type"] == "world" {
			world = msg
		}
	}
	if world["winner"] != "draw" {
		t.Fatal("reconnect lost result")
	}
	l.gameInput(c, command{Type: "move", MatchID: m.ID, Sequence: 1, Position: &point{3000, 0}})
	var reply map[string]any
	json.Unmarshal(<-c.send, &reply)
	if reply["ok"] != false {
		t.Fatal("finished input accepted")
	}
	l.tick(time.Unix(302, 0))
	if l.matches[m.ID] != nil {
		t.Fatal("finished match not expired")
	}
}
func TestPushAllStructuresAndSingleReward(t *testing.T) {
	g := laneGame()
	a := g.Actors[0]
	a.Character = "Chiyo"
	for _, id := range []string{"red-outer", "red-inner", "red-base"} {
		target := g.actor(id)
		a.Position = point{target.Position.S - 150, 0}
		supporter := minion(g, "support-"+id, "blue", point{target.Position.S - 140, 0})
		g.unlockStructures()
		if !target.unlocked {
			t.Fatal("next structure locked", id)
		}
		a.attackTarget = id
		a.windupUntil = float64(g.Step+1) * .05
		a.attackDamage = 5000
		supporter.attackTarget = id
		supporter.windupUntil = a.windupUntil
		supporter.attackDamage = 5000
		g.advance()
		if target.HP != 0 {
			t.Fatal("failed to destroy", id)
		}
		if id != "red-base" && g.Winner != "" {
			t.Fatal("premature victory")
		}
	}
	if g.Winner != "blue" || a.Gold != 500 {
		t.Fatal("victory or duplicated tower reward", g.Winner, a.Gold)
	}
}
func TestLeashAndFirstLethalAttribution(t *testing.T) {
	g := laneGame()
	a := g.Actors[0]
	b := g.Actors[1]
	a.Position = point{3000, 0}
	b.Position = point{3200, 0}
	m := minion(g, "leashed", "blue", point{3100, 0})
	g.selectTarget(m, b)
	m.anchor = point{2500, 0}
	m.lane = 60
	g.advanceNPC()
	if m.attackTarget != "" || len(m.path) == 0 {
		t.Fatal("leash did not return to lane")
	}
	victim := minion(g, "victim", "red", point{3050, 0})
	victim.HP = 10
	a.Character = "Chiyo"
	a.attackTarget = victim.ID
	a.windupUntil = .01
	a.attackDamage = 100
	m.Position = point{3000, 0}
	m.attackTarget = victim.ID
	m.windupUntil = .02
	m.attackDamage = 100
	g.Step = 1
	g.combat()
	if a.LastHits != 1 || a.Gold != 315 || a.XP != 20 {
		t.Fatal("first lethal event or rewards incorrect", a.LastHits, a.Gold, a.XP)
	}
	g.Step++
	g.combat()
	if a.LastHits != 1 || a.Gold != 315 {
		t.Fatal("duplicate reward on next tick")
	}
	a.attackTarget = victim.ID
	a.path = []point{{4000, 0}}
	g.advanceNPC()
	g.combat()
	if a.attackTarget != "" || len(a.path) != 0 {
		t.Fatal("removed minion left stale chase order")
	}
}
func TestLaneSimulationTenMinutes(t *testing.T) {
	g := laneGame()
	start := time.Now()
	maxMinions := 0
	for i := 0; i < 12000; i++ {
		g.advance()
		maxMinions = max(maxMinions, len(g.Minions))
		for _, a := range g.Minions {
			if a.HP > 0 && !g.navigation(a).legal(a.Position) {
				t.Fatal("illegal NPC position", g.Step, a.ID, a.Position)
			}
		}
	}
	t.Logf("10 simulated minutes: %s, maximum %d minions, winner %q", time.Since(start), maxMinions, g.Winner)
	if maxMinions > 200 {
		t.Fatal("minions are not fighting/removing dead units", maxMinions)
	}
	if g.Actors[0].Kills != 0 || g.Actors[1].Kills != 0 {
		t.Fatal("idle players gained NPC kill credit")
	}
}
