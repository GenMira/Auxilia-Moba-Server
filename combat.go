package main

import "math"

type projectile struct {
	ID        int    `json:"id"`
	Owner     string `json:"owner"`
	Target    string `json:"target"`
	Position  point  `json:"position"`
	damage    float64
	spawnStep int64
}
type hit struct {
	owner, target *avatar
	damage        float64
}

func (g *game) actor(id string) *avatar {
	for _, a := range g.Actors {
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
}
func (g *game) combat() {
	seconds := float64(g.Step) * .05
	hits := []hit{}
	for _, a := range g.Actors {
		if a.HP <= 0 {
			continue
		}
		target := g.actor(a.attackTarget)
		if target == nil {
			continue
		}
		if target.HP <= 0 || !g.visible(a, target) {
			a.stop()
			continue
		}
		if a.windupUntil > 0 {
			if seconds+1e-8 >= a.windupUntil {
				a.windupUntil = 0
				if distance(a.Position, target.Position) <= a.Stats.Range {
					if a.Character == "Sophie" {
						g.projectileID++
						g.Projectiles = append(g.Projectiles, projectile{ID: g.projectileID, Owner: a.ID, Target: target.ID, Position: a.Position, damage: a.attackDamage, spawnStep: g.Step})
					} else {
						hits = append(hits, hit{a, target, a.attackDamage})
					}
				}
			}
			continue
		}
		if distance(a.Position, target.Position) > a.Stats.Range {
			if g.Step%5 == 0 || len(a.path) == 0 {
				a.path = findPath(a.Position, target.Position)
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
		target := g.actor(p.Target)
		owner := g.actor(p.Owner)
		if target == nil || target.HP <= 0 {
			continue
		}
		d := distance(p.Position, target.Position)
		travel := math.Min(1800*.05, d)
		next := target.Position
		if d > travel {
			next = point{p.Position.S + (target.Position.S-p.Position.S)*travel/d, p.Position.T + (target.Position.T-p.Position.T)*travel/d}
		}
		blocked := false
		for _, o := range arena.Structures {
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
			hits = append(hits, hit{owner, target, p.damage})
		} else {
			alive = append(alive, p)
		}
	}
	g.Projectiles = alive
	killers := map[string]*avatar{}
	for _, h := range hits {
		if h.target.HP <= 0 {
			continue
		}
		h.target.HP -= h.damage
		h.target.RecallUntil = 0
		h.target.HitAt = seconds
		if h.target.HP <= 0 {
			killers[h.target.ID] = h.owner
		}
	}
	// Freeze death times before awarding same-tick kills, allowing mutual kills.
	for _, a := range g.Actors {
		if _, dead := killers[a.ID]; dead {
			a.HP = 0
			a.Deaths++
			a.RespawnAt = seconds + math.Max(5, float64(a.Kills)*5)
			a.stop()
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
