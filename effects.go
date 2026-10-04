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
	if a.isStructure() {
		return
	}
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
	for _, a := range g.units() {
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
	if h.target.isStructure() {
		if h.source != "attack" || h.owner == nil || h.owner.isStructure() || !h.target.unlocked {
			return damageResult{}
		}
		if h.owner.isChampion() && !g.hasMinionSupport(h.owner.Team, h.target.Position) {
			amount *= .2
		}
		amount = math.Max(0, amount)
		result := damageResult{mitigated: amount, hpDamage: math.Min(h.target.HP, amount)}
		h.target.HP = math.Max(0, h.target.HP-amount)
		h.target.HitAt = now
		return result
	}
	if h.source == "attack" && h.owner != nil && h.owner.Character == "Nadia" {
		h.owner.AttackCount = (h.owner.AttackCount + 1) % 3
		if h.owner.AttackCount == 0 {
			amount += h.damage * .5
		}
	}
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
	kept := h.target.Statuses[:0]
	for _, s := range h.target.Statuses {
		if s.Kind != "shield" || s.Value > 0 {
			kept = append(kept, s)
		}
	}
	h.target.Statuses = kept
	result.hpDamage = math.Min(math.Max(0, h.target.HP), amount)
	h.target.HP = math.Max(0, h.target.HP-amount)
	if result.absorbed+result.hpDamage > 0 && h.owner != nil && h.owner.isChampion() && h.target.isChampion() {
		h.target.lastDamager = h.owner.ID
		h.target.lastDamageAt = now
		g.protect(h.owner, h.target, now)
	}
	if h.source == "attack" || h.source == "skill" {
		if h.poison && h.owner != nil {
			h.target.addStatus(statusEffect{ID: "nadia-poison", Source: h.owner.ID, Kind: "poison", Until: now + 3, nextTick: now + 1, interval: 1, damage: 8})
		}
		if h.owner != nil && h.owner.Character == "Sophie" && h.target.isChampion() && h.owner.HP > 0 {
			h.owner.addStatus(statusEffect{ID: "sowing", Source: h.owner.ID, Kind: "speed", Until: now + 2, Value: 30})
		}
		if h.slow > 0 {
			h.target.addStatus(statusEffect{ID: h.effectID, Source: h.owner.ID, Kind: "slow", Until: now + h.slowDuration, Value: h.slow})
		}
	}
	return result
}
