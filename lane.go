package main

import (
	"fmt"
	"math"
	"strings"
)

func (a *avatar) isChampion() bool  { return a.Kind == "" }
func (a *avatar) isStructure() bool { return a.Kind == "tower" || a.Kind == "base" }
func (a *avatar) radius() float64 {
	switch a.Kind {
	case "minion":
		return 20
	case "tower":
		return 75
	case "base":
		return 100
	}
	return arena.Radius
}
func (g *game) units() []*avatar {
	out := make([]*avatar, 0, 2+len(g.Minions))
	out = append(out, g.Actors[:]...)
	return append(out, g.Minions...)
}
func (g *game) entities() []*avatar { return append(g.units(), g.Structures...) }
func (g *game) liveStructures() []structure {
	out := []structure{}
	for _, a := range g.Structures {
		if a.HP > 0 {
			out = append(out, structure{a.ID, a.Team, a.Kind, a.Position, a.radius(), a.HP})
		}
	}
	return out
}

// Called once before each simulation update, never between damage events.
func (g *game) unlockStructures() {
	for _, a := range g.Structures {
		preceding := ""
		if a.Kind == "base" {
			preceding = a.Team + "-inner"
		} else if strings.HasSuffix(a.ID, "inner") {
			preceding = a.Team + "-outer"
		}
		a.unlocked = a.HP > 0 && (preceding == "" || g.actor(preceding).HP <= 0)
	}
}
func (g *game) hasMinionSupport(team string, p point) bool {
	for _, a := range g.Minions {
		if a.HP > 0 && a.Team == team && distance(a.Position, p) <= 900 {
			return true
		}
	}
	return false
}
func (g *game) selectTarget(a, target *avatar) {
	id := ""
	if target != nil {
		id = target.ID
	}
	if a.attackTarget == id {
		return
	}
	a.attackTarget = id
	a.windupUntil = 0
	a.shots = 0
	a.path = nil
	a.anchor = a.Position
}
func (g *game) validNPCTarget(a, target *avatar) bool {
	if target == nil || target.HP <= 0 || target.Team == a.Team || !g.visible(a, target) {
		return false
	}
	if target.isStructure() && !target.unlocked {
		return false
	}
	limit := 600.0
	if a.Kind == "tower" {
		limit = 750
		if target.isStructure() {
			return false
		}
	}
	return distance(a.Position, target.Position) <= limit && (a.Kind != "minion" || distance(a.Position, a.anchor) < 600)
}
func targetPriority(a *avatar) int {
	switch a.Kind {
	case "minion":
		return 0
	case "tower":
		return 2
	case "base":
		return 3
	}
	return 1
}
func (g *game) acquire(a *avatar) *avatar {
	var best *avatar
	for _, candidate := range g.entities() {
		if !g.validNPCTarget(a, candidate) {
			continue
		}
		if best == nil || targetPriority(candidate) < targetPriority(best) || (targetPriority(candidate) == targetPriority(best) && (distance(a.Position, candidate.Position) < distance(a.Position, best.Position) || (distance(a.Position, candidate.Position) == distance(a.Position, best.Position) && candidate.ID < best.ID))) {
			best = candidate
		}
	}
	return best
}
func (g *game) advanceNPC() {
	if g.disableNPC {
		return
	}
	now := float64(g.Step) * .05
	// Remove dead minions on the following update, after rewards and impact attribution.
	kept := g.Minions[:0]
	for _, a := range g.Minions {
		if a.HP > 0 {
			kept = append(kept, a)
		}
	}
	g.Minions = kept
	if g.Step >= 200 && (g.Step-200)%600 == 0 {
		for _, team := range []string{"blue", "red"} {
			start, dir := 600.0, 1.0
			if team == "red" {
				start = 5400
				dir = -1
			}
			for i := 0; i < 10; i++ {
				lane := -60.0
				if i%2 == 1 {
					lane = 60
				}
				p := point{start + dir*float64(i/2)*60, lane}
				g.Minions = append(g.Minions, &avatar{ID: fmt.Sprintf("minion-%s-%06d-%02d", team, (g.Step-200)/600, i), Team: team, Kind: "minion", Name: "工作員", Position: p, Facing: point{dir, 0}, HP: 100, Stats: characterDefinition{HP: 100, Attack: 20, Range: 150, AttackSpeed: 1, Speed: 300}, lane: lane, anchor: p, spawnStep: g.Step, Statuses: []statusEffect{}})
			}
		}
	}
	for _, a := range append(append([]*avatar{}, g.Minions...), g.Structures...) {
		if a.HP <= 0 || a.Kind == "base" {
			continue
		}
		target := g.actor(a.attackTarget)
		if a.Kind == "minion" && target != nil && distance(a.Position, a.anchor) >= 600-1e-8 {
			g.selectTarget(a, nil)
			a.guardUntil = 0
			dir := 1.0
			if a.Team == "red" {
				dir = -1
			}
			a.path = g.navigation(a).findPath(a.Position, point{a.Position.S + dir*150, a.lane})
			g.moveActor(a, now)
			continue
		}
		if !g.validNPCTarget(a, target) || (a.Kind == "minion" && a.guardUntil > 0 && now >= a.guardUntil) {
			g.selectTarget(a, nil)
			a.guardUntil = 0
			a.anchor = a.Position
			g.selectTarget(a, g.acquire(a))
		}
		if a.Kind == "minion" {
			if a.attackTarget == "" && len(a.path) == 0 {
				end := 5800.0
				if a.Team == "red" {
					end = 200
				}
				a.path = g.navigation(a).findPath(a.Position, point{end, a.lane})
			}
			if a.spawnStep != g.Step {
				g.moveActor(a, now)
			}
		}
	}
}

// Positive champion damage (including shield absorption and DoT) invokes guards.
func (g *game) protect(attacker, victim *avatar, now float64) {
	if g.disableNPC {
		return
	}
	for _, a := range g.Minions {
		if a.HP > 0 && a.Team == victim.Team && distance(a.Position, victim.Position) <= 150 && distance(a.Position, attacker.Position) <= 600 && g.visible(a, attacker) {
			g.selectTarget(a, attacker)
			a.guardUntil = now + 3
		}
	}
	for _, a := range g.Structures {
		if a.Kind == "tower" && a.HP > 0 && a.Team == victim.Team && distance(a.Position, victim.Position) <= 750 && distance(a.Position, attacker.Position) <= 750 && g.visible(a, attacker) {
			g.selectTarget(a, attacker)
		}
	}
}
func (g *game) rewardNPCDeaths(killers map[string]*avatar) {
	// Iterate stable entity order, not Go map iteration order.
	for _, dead := range g.entities() {
		killer, ok := killers[dead.ID]
		if !ok || dead.isChampion() {
			continue
		}
		dead.HP = 0
		dead.stop()
		for _, a := range g.Actors {
			if a.Team == dead.Team {
				continue
			}
			if dead.Kind == "tower" {
				a.Gold += 100
			}
			if dead.Kind != "minion" {
				continue
			}
			nearby := a.HP > 0 && distance(a.Position, dead.Position) <= 1400
			if killer == a {
				a.Gold += 15
				a.LastHits++
			} else if nearby {
				a.Gold += 5
			}
			if nearby {
				a.XP += 20
				g.levelUp(a)
			}
		}
	}
}
func (g *game) checkVictory() {
	blue, red := g.actor("blue-base"), g.actor("red-base")
	if blue == nil || red == nil {
		return
	}
	if blue.HP <= 0 && red.HP <= 0 {
		g.Winner = "draw"
	} else if blue.HP <= 0 {
		g.Winner = "red"
	} else if red.HP <= 0 {
		g.Winner = "blue"
	}
	if g.Winner != "" {
		for _, a := range g.entities() {
			a.stop()
			a.Cast = nil
		}
		g.Projectiles = nil
	}
}

type structureView struct {
	structure
	MaxHP      float64 `json:"maxHp"`
	Visible    bool    `json:"visible"`
	Destroyed  bool    `json:"destroyed"`
	Attackable bool    `json:"attackable"`
}

func (g *game) mapView(viewer *avatar) any {
	if g.lastSeen == nil {
		g.lastSeen = map[string]map[string]float64{}
	}
	if g.lastSeen[viewer.Team] == nil {
		g.lastSeen[viewer.Team] = map[string]float64{}
	}
	seen := g.lastSeen[viewer.Team]
	out := []structureView{}
	for _, a := range g.Structures {
		visible := g.visible(viewer, a)
		hp, known := seen[a.ID]
		if !known {
			hp = a.Stats.HP
		}
		if visible || a.HP <= 0 {
			hp = math.Max(0, a.HP)
			seen[a.ID] = hp
		}
		out = append(out, structureView{structure{a.ID, a.Team, a.Kind, a.Position, a.radius(), hp}, a.Stats.HP, visible, a.HP <= 0, a.unlocked && a.HP > 0})
	}
	return struct {
		mapDefinition
		Structures []structureView `json:"structures"`
	}{arena, out}
}

// Avoid sending champion-only mana, progression, spells and cooldown fields for every NPC.
type minionView struct {
	ID       string  `json:"id"`
	Team     string  `json:"team"`
	Kind     string  `json:"kind"`
	Name     string  `json:"name"`
	Position point   `json:"position"`
	Facing   point   `json:"facing"`
	HP       float64 `json:"hp"`
	Stats    struct {
		HP float64 `json:"hp"`
	} `json:"stats"`
	HitAt       float64        `json:"hitAt"`
	AttackUntil float64        `json:"attackUntil"`
	Statuses    []statusEffect `json:"statuses"`
}

func (g *game) minionViews(viewer *avatar) []minionView {
	out := []minionView{}
	for _, a := range g.Minions {
		if a.HP > 0 && g.visible(viewer, a) {
			c := minionView{ID: a.ID, Team: a.Team, Kind: a.Kind, Name: a.Name, Position: a.Position, Facing: a.Facing, HP: a.HP, HitAt: a.HitAt, AttackUntil: a.windupUntil, Statuses: a.activeStatuses(float64(g.Step) * .05)}
			c.Stats.HP = a.Stats.HP
			out = append(out, c)
		}
	}
	return out
}
