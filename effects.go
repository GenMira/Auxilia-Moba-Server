package main

import "math"

type statusEffect struct {
	ID                         string  `json:"id"`
	Source                     string  `json:"source"`
	Kind                       string  `json:"kind"`
	Until                      float64 `json:"until"`
	Value                      float64 `json:"value"`
	nextTick, interval, damage float64
	trueDamage                 bool
}

func (a *avatar) hasStatus(kind string, now float64) bool {
	for _, s := range a.Statuses {
		if s.Kind == kind && s.Until > now {
			return true
		}
	}
	return false
}
func (a *avatar) activeStatuses(now float64) []statusEffect {
	out := []statusEffect{}
	for _, s := range a.Statuses {
		if s.Until > now {
			out = append(out, s)
		}
	}
	return out
}
func (a *avatar) addStatus(effect statusEffect) {
	for i, s := range a.Statuses {
		if s.ID == effect.ID && s.Source == effect.Source {
			if effect.Kind != "shield" {
				effect.nextTick = s.nextTick
			}
			a.Statuses[i] = effect
			return
		}
	}
	a.Statuses = append(a.Statuses, effect)
}
func (a *avatar) cleanse(kinds ...string) {
	out := a.Statuses[:0]
	for _, s := range a.Statuses {
		remove := false
		for _, kind := range kinds {
			if s.Kind == kind {
				remove = true
			}
		}
		if !remove {
			out = append(out, s)
		}
	}
	a.Statuses = out
}
func (a *avatar) moveSpeed(now float64) float64 {
	bonus, slow := 0.0, 0.0
	for _, s := range a.Statuses {
		if s.Until <= now {
			continue
		}
		if s.Kind == "speed" {
			bonus += s.Value
		}
		if s.Kind == "slow" {
			slow = math.Max(slow, s.Value)
		}
	}
	return math.Max(150, math.Min(500, (a.Stats.Speed+bonus)*(1-slow)))
}
func (g *game) statusTicks() []hit {
	now := float64(g.Step) * .05
	hits := []hit{}
	for _, a := range g.Actors {
		if a.HP <= 0 {
			continue
		}
		kept := a.Statuses[:0]
		for _, s := range a.Statuses {
			for s.interval > 0 && s.nextTick > 0 && s.nextTick <= now+1e-8 && s.nextTick <= s.Until+1e-8 {
				h := g.newHit(g.actor(s.Source), a, s.damage, s.Kind, s.nextTick)
				h.trueDamage = s.trueDamage
				hits = append(hits, h)
				s.nextTick += s.interval
			}
			if s.Until > now {
				kept = append(kept, s)
			}
		}
		a.Statuses = kept
	}
	return hits
}
func (g *game) newHit(owner, target *avatar, amount float64, source string, at float64) hit {
	g.eventID++
	return hit{owner: owner, target: target, damage: amount, source: source, at: at, id: g.eventID}
}

// DoT/item events cannot recursively proc direct-hit passives. Future item
// follow-ups consume hpDamage, after mitigation and shield absorption.
type damageResult struct{ mitigated, absorbed, hpDamage float64 }

func (g *game) applyDamage(h hit) damageResult {
	now := float64(g.Step) * .05
	amount := h.damage
	if !h.trueDamage {
		amount *= math.Max(0, 1+h.outgoingBonus)
		incoming := 0.0
		for _, s := range h.target.Statuses {
			if s.Kind == "damageTaken" && s.Until > now {
				incoming += s.Value
			}
		}
		amount *= math.Max(0, 1+incoming)
		if h.target.Character == "Jude" && h.source != "poison" {
			amount -= 10
		}
	}
	amount = math.Max(0, amount)
	result := damageResult{mitigated: amount}
	if amount > 0 {
		h.target.RecallUntil = 0
		h.target.HitAt = now
	}
	for i := range h.target.Statuses {
		s := &h.target.Statuses[i]
		if s.Kind == "shield" && s.Until > now {
			used := math.Min(amount, s.Value)
			s.Value -= used
			amount -= used
			result.absorbed += used
		}
	}
	result.hpDamage = math.Min(math.Max(0, h.target.HP), amount)
	h.target.HP -= amount
	if h.source == "attack" || h.source == "skill" {
		if h.owner != nil && h.owner.Character == "Sophie" && h.owner.HP > 0 {
			h.owner.addStatus(statusEffect{ID: "sowing", Source: h.owner.ID, Kind: "speed", Until: now + 2, Value: 30})
		}
		if h.slow > 0 {
			h.target.addStatus(statusEffect{ID: h.effectID, Source: h.owner.ID, Kind: "slow", Until: now + h.slowDuration, Value: h.slow})
		}
	}
	return result
}
