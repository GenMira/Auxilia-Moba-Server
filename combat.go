package main

import (
	"math"
	"sort"
)

type projectile struct {
	Kind      string  `json:"kind"`
	Direction point   `json:"direction"`
	Remaining float64 `json:"remaining"`
	Width     float64 `json:"width"`
	ID        int     `json:"id"`
	Owner     string  `json:"owner"`
	Target    string  `json:"target"`
	Position  point   `json:"position"`
	damage    float64
	spawnStep int64
}
type hit struct {
	poison             bool
	owner, target      *avatar
	damage             float64
	source             string
	trueDamage         bool
	outgoingBonus      float64
	at                 float64
	id                 uint64
	slow, slowDuration float64
	effectID           string
}

func (g *game) actor(id string) *avatar {
	for _, a := range g.Actors {
		if a != nil && a.ID == id {
			return a
		}
	}
	for _, a := range g.Minions {
		if a.ID == id {
			return a
		}
	}
	for _, a := range g.Structures {
		if a.ID == id {
			return a
		}
	}
	return nil
}
func (a *avatar) stop() {
	a.path = nil
	a.Moving = false
	a.attackTarget = ""
	a.windupUntil = 0
	a.RecallUntil = 0
	a.shots = 0
}
func (g *game) combat() {
	seconds := float64(g.Step) * .05
	hits := g.resolveSkills()
	hits = append(hits, g.statusTicks()...)
	for _, a := range g.entities() {
		if a.Kind == "base" || (g.disableNPC && !a.isChampion()) {
			continue
		}
		if a.HP <= 0 || a.Cast != nil || a.hasStatus("stun", seconds) {
			continue
		}
		target := g.actor(a.attackTarget)
		if target == nil {
			if a.attackTarget != "" {
				a.stop()
			}
			continue
		}
		if target.HP <= 0 || (target.isStructure() && !target.unlocked) || !g.visible(a, target) {
			a.stop()
			continue
		}
		if a.windupUntil > 0 {
			if seconds+1e-8 >= a.windupUntil {
				impactAt := a.windupUntil
				a.windupUntil = 0
				if distance(a.Position, target.Position) <= a.Stats.Range {
					if a.Character == "Sophie" || a.Kind == "tower" {
						kind := ""
						damage := a.attackDamage
						if a.Kind == "tower" {
							kind = "tower"
							a.shots++
							damage = 100 * float64(a.shots)
						}
						g.projectileID++
						g.Projectiles = append(g.Projectiles, projectile{Kind: kind, ID: g.projectileID, Owner: a.ID, Target: target.ID, Position: a.Position, damage: damage, spawnStep: g.Step})
					} else {
						h := g.newHit(a, target, a.attackDamage, "attack", impactAt)
						h.outgoingBonus = a.attackBonus
						hits = append(hits, h)
					}
				}
			}
			continue
		}
		if distance(a.Position, target.Position) > a.Stats.Range {
			if g.Step%5 == 0 || len(a.path) == 0 {
				if !a.isStructure() {
					a.path = g.navigation(a).findPath(a.Position, target.Position)
				}
			}
			continue
		}
		a.path = nil
		a.Moving = false
		if seconds+1e-8 >= a.nextAttack {
			if bushAt(a.Position) >= 0 {
				a.revealedUntil = seconds + 2
			}
			interval := 1 / a.Stats.AttackSpeed
			a.nextAttack = seconds + interval
			a.windupUntil = seconds + interval*.3
			a.attackDamage = a.Stats.Attack
			a.attackBonus = a.chiyoBonus("attack")
			d := distance(a.Position, target.Position)
			if d > 0 {
				a.Facing = point{(target.Position.S - a.Position.S) / d, (target.Position.T - a.Position.T) / d}
			}
		}
	}
	alive := g.Projectiles[:0]
	for _, p := range g.Projectiles {
		if p.spawnStep == g.Step {
			alive = append(alive, p)
			continue
		}
		if p.Kind == "skill" {
			travel := math.Min(1600*.05, p.Remaining)
			obstacle := g.navigation(g.Actors[0]).obstacleDistance(p.Position, p.Direction, math.Inf(1), p.Width/2)
			contact := math.Inf(1)
			var victim *avatar
			owner := g.actor(p.Owner)
			for _, a := range g.units() {
				if a.HP <= 0 || owner == nil || a.Team == owner.Team {
					continue
				}
				d := rayCircle(p.Position, p.Direction, a.Position, p.Width/2+a.radius())
				if d < contact || (d == contact && victim != nil && a.ID < victim.ID) {
					contact = d
					victim = a
				}
			}
			if victim != nil && contact <= travel && contact < obstacle {
				hits = append(hits, g.newHit(owner, victim, p.damage, "skill", seconds-.05+contact/1600))
				continue
			}
			if obstacle <= travel {
				continue
			}
			p.Position = point{p.Position.S + p.Direction.S*travel, p.Position.T + p.Direction.T*travel}
			p.Remaining -= travel
			if p.Remaining > 1e-8 {
				alive = append(alive, p)
			}
			continue
		}
		target := g.actor(p.Target)
		owner := g.actor(p.Owner)
		if target == nil || target.HP <= 0 {
			continue
		}
		d := distance(p.Position, target.Position)
		speed := 1800.0
		if p.Kind == "tower" {
			speed = 2000
		}
		travel := math.Min(speed*.05, d)
		next := target.Position
		if d > travel {
			next = point{p.Position.S + (target.Position.S-p.Position.S)*travel/d, p.Position.T + (target.Position.T-p.Position.T)*travel/d}
		}
		blocked := false
		for _, o := range g.liveStructures() {
			if p.Kind == "tower" || o.ID == p.Target || o.ID == p.Owner {
				continue
			}
			if segmentDistance(p.Position, next, o.Position) < o.Radius {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		p.Position = next
		if d <= travel {
			hits = append(hits, g.newHit(owner, target, p.damage, "attack", seconds-.05+d/speed))
		} else {
			alive = append(alive, p)
		}
	}
	g.Projectiles = alive
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if math.Abs(a.at-b.at) > 1e-8 {
			return a.at < b.at
		}
		aid, bid := "", ""
		if a.owner != nil {
			aid = a.owner.ID
		}
		if b.owner != nil {
			bid = b.owner.ID
		}
		if aid != bid {
			return aid < bid
		}
		return a.id < b.id
	})
	killers := map[string]*avatar{}
	for _, h := range hits {
		if h.target.HP <= 0 {
			continue
		}
		g.applyDamage(h)
		if h.target.HP <= 0 {
			killer := h.owner
			if h.target.isChampion() && (killer == nil || !killer.isChampion()) {
				killer = nil
				if seconds-h.target.lastDamageAt <= 10 && h.target.lastDamager != "" {
					killer = g.actor(h.target.lastDamager)
				}
			}
			killers[h.target.ID] = killer
		}
	}
	g.rewardNPCDeaths(killers)
	// Freeze death times before awarding same-tick kills, allowing mutual kills.
	for _, a := range g.Actors {
		if _, dead := killers[a.ID]; dead {
			a.HP = 0
			a.Deaths++
			a.RespawnAt = seconds + math.Max(5, float64(a.Kills)*5)
			a.stop()
			a.Cast = nil
			a.Statuses = []statusEffect{}
			a.AttackCount = 0
			a.lastDamager = ""
			a.lastDamageAt = 0
		}
	}
	for _, a := range g.Actors {
		killer, dead := killers[a.ID]
		if !dead {
			continue
		}
		if killer != nil {
			killer.Gold += max(100, a.streak*100)
			killer.Kills++
			if killer.HP > 0 {
				killer.streak++
			}
			killer.XP += 100
			g.levelUp(killer)
		}
	}
	for _, a := range g.Actors {
		if _, dead := killers[a.ID]; dead {
			a.streak = 0
		}
	}
	g.checkVictory()
}
func segmentDistance(a, b, c point) float64 {
	dx, dy := b.S-a.S, b.T-a.T
	q := dx*dx + dy*dy
	t := 0.0
	if q > 0 {
		t = math.Max(0, math.Min(1, ((c.S-a.S)*dx+(c.T-a.T)*dy)/q))
	}
	return distance(point{a.S + dx*t, a.T + dy*t}, c)
}
func (g *game) levelUp(a *avatar) {
	growth := map[string][3]float64{"Sophie": {70, 35, 4}, "Jude": {90, 25, 4}, "Nadia": {80, 30, 5}, "Chiyo": {50, 30, 4}}[a.Character]
	for a.Level < 7 && a.XP >= 100*a.Level {
		a.XP -= 100 * a.Level
		a.Level++
		a.SkillPoints++
		a.Stats.HP += growth[0]
		a.Stats.Mana += growth[1]
		a.Stats.Attack += growth[2]
		if a.HP > 0 {
			a.HP += growth[0]
			a.Mana += growth[1]
		}
	}
	if a.Level == 7 {
		a.XP = 0
	}
}
