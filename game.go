package main

import (
	"math"
	"time"
)

type avatar struct {
	Spells         []skillView `json:"spells,omitempty"`
	AttackCount    int         `json:"attackCount"`
	spellIDs       [2]string
	spellCD        [2]float64
	attackBonus    float64
	Skills         []skillView    `json:"skills,omitempty"`
	Cast           *castState     `json:"cast"`
	Statuses       []statusEffect `json:"statuses"`
	MoveSpeed      float64        `json:"moveSpeed"`
	SkillPower     float64        `json:"skillPower"`
	ranks          [3]int
	cooldowns      [3]float64
	ID             string              `json:"id"`
	Team           string              `json:"team"`
	Character      string              `json:"character"`
	Name           string              `json:"name"`
	Position       point               `json:"position"`
	Facing         point               `json:"facing"`
	Stats          characterDefinition `json:"stats"`
	HP             float64             `json:"hp"`
	Mana           float64             `json:"mana"`
	Level          int                 `json:"level"`
	Gold           int                 `json:"gold"`
	Moving         bool                `json:"moving"`
	RecallUntil    float64             `json:"recallUntil"`
	Disconnected   bool                `json:"disconnected"`
	Kills          int                 `json:"kills"`
	Deaths         int                 `json:"deaths"`
	XP             int                 `json:"xp"`
	SkillPoints    int                 `json:"skillPoints"`
	RespawnAt      float64             `json:"respawnAt"`
	HitAt          float64             `json:"hitAt"`
	AttackTarget   string              `json:"attackTarget"`
	AttackUntil    float64             `json:"attackUntil"`
	path           []point
	sequence       uint64
	disconnectedAt time.Time
	attackTarget   string
	nextAttack     float64
	windupUntil    float64
	attackDamage   float64
	streak         int
	revealedUntil  float64
}
type game struct {
	Effects      []skillEffect
	eventID      uint64
	Actors       [2]*avatar
	Step         int64
	lastStep     time.Time
	Projectiles  []projectile
	projectileID int
}

func spawn(team string) point {
	if team == "blue" {
		return point{200, 0}
	}
	return point{5800, 0}
}
func newGame(m *match, now time.Time) *game {
	g := &game{lastStep: now}
	for i, p := range m.Players {
		stats := characterDefinitions[p.Character]
		facing := point{1, 0}
		if p.Team == "red" {
			facing.S = -1
		}
		g.Actors[i] = &avatar{ID: p.ID, Team: p.Team, Character: p.Character, Name: p.Name, Position: spawn(p.Team), Facing: facing, Stats: stats, HP: stats.HP, Mana: stats.Mana, Level: 1, Gold: 300}
		g.Actors[i].ranks = [3]int{1, 1, 1}
		copy(g.Actors[i].spellIDs[:], p.Spells)
		g.Actors[i].Statuses = []statusEffect{}
	}
	return g
}
func (g *game) visible(viewer, target *avatar) bool {
	if viewer.ID == target.ID {
		return true
	}
	if target.HP <= 0 {
		return false
	}
	if target.revealedUntil > float64(g.Step)*.05 {
		return true
	}
	targetBush := bushAt(target.Position)
	if viewer.HP > 0 && distance(viewer.Position, target.Position) <= 1200 && (targetBush < 0 || bushAt(viewer.Position) == targetBush) {
		return true
	}
	for _, o := range arena.Structures {
		if o.Team != viewer.Team {
			continue
		}
		radius := 1000.0
		if o.Kind == "base" {
			radius = 800
		}
		if distance(o.Position, target.Position) <= radius && (targetBush < 0 || bushAt(o.Position) == targetBush) {
			return true
		}
	}
	return false
}
func (g *game) advance() {
	g.Step++
	seconds := float64(g.Step) * .05
	g.interruptCasts(seconds)
	for _, a := range g.Actors {
		if a.HP <= 0 {
			if g.Step >= 200 && g.Step%20 == 0 {
				a.Gold += 2
			}
			if seconds+1e-8 >= a.RespawnAt {
				a.Position = spawn(a.Team)
				a.HP = a.Stats.HP
				a.Mana = a.Stats.Mana
				a.RespawnAt = 0
				a.stop()
			}
			continue
		}
		a.HP = math.Min(a.Stats.HP, a.HP+a.Stats.HPRegen*.05)
		a.Mana = math.Min(a.Stats.Mana, a.Mana+a.Stats.ManaRegen*.05)
		if g.Step >= 200 && g.Step%20 == 0 {
			a.Gold += 2
		}
		remaining := a.moveSpeed(seconds) * .05
		if a.Cast != nil || a.hasStatus("stun", seconds) || a.hasStatus("root", seconds) {
			remaining = 0
		}
		for len(a.path) > 0 && remaining > 0 {
			target := a.path[0]
			d := distance(a.Position, target)
			if d < .001 {
				a.path = a.path[1:]
				continue
			}
			travel := math.Min(remaining, d)
			a.Facing = point{(target.S - a.Position.S) / d, (target.T - a.Position.T) / d}
			a.Position.S += a.Facing.S * travel
			a.Position.T += a.Facing.T * travel
			remaining -= travel
			if travel >= d {
				a.Position = target
				a.path = a.path[1:]
			}
		}
		a.Moving = len(a.path) > 0
	}
	g.combat()
	// Damage on the completion tick still interrupts recall.
	for _, a := range g.Actors {
		if a.HP > 0 && a.RecallUntil > 0 && seconds+1e-8 >= a.RecallUntil {
			a.Position = spawn(a.Team)
			a.stop()
		}
	}
}
func (l *lobby) snapshot(s *session, m *match) {
	if m.Game == nil {
		return
	}
	g := m.Game
	var self *avatar
	for _, a := range g.Actors {
		if a.ID == s.id {
			self = a
		}
	}
	if self == nil {
		return
	}
	actors := []avatar{}
	for _, a := range g.Actors {
		if g.visible(self, a) {
			copy := *a
			copy.MoveSpeed = a.moveSpeed(float64(g.Step) * .05)
			copy.Statuses = a.activeStatuses(float64(g.Step) * .05)
			if a.ID == self.ID {
				copy.Skills = g.skillViews(a)
				copy.Spells = g.spellViews(a)
			}
			copy.AttackUntil = a.windupUntil
			if a.ID == self.ID {
				copy.AttackTarget = a.attackTarget
			}
			actors = append(actors, copy)
		}
	}
	projectiles := []projectile{}
	for _, p := range g.Projectiles {
		probe := &avatar{Position: p.Position, HP: 1}
		if g.visible(self, probe) {
			projectiles = append(projectiles, p)
		}
	}
	effects := []skillEffect{}
	for _, e := range g.Effects {
		if e.Owner == self.ID || g.visible(self, &avatar{Position: e.Origin, HP: 1}) {
			effects = append(effects, e)
		}
	}
	message := map[string]any{"effects": effects, "projectiles": projectiles, "type": "world", "matchId": m.ID, "time": float64(g.Step) * .05, "ack": self.sequence, "actors": actors, "map": arena}
	for c := range s.connections {
		l.emit(c, message)
	}
}
