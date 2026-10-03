package main

import "math"

var spellDefinitions = map[string]skillDefinition{
	"flash":   {Name: "フラッシュ", Aim: "point", Shape: "blink", Range: 400, Cooldown: 120, Description: "カーソルへ最大400U瞬間移動。施設を飛び越せる。"},
	"ignite":  {Name: "イグナイト", Aim: "target", Shape: "target", Range: 600, Cooldown: 70, Description: "敵キャラに毎秒10の確定ダメージ、5回。毒解除では消えない。"},
	"barrier": {Name: "バリア", Aim: "self", Shape: "shield", Cooldown: 90, Description: "100ダメージを吸収するシールド、5秒。"},
}

func spellIndex(slot string) int {
	switch slot {
	case "d":
		return 0
	case "f":
		return 1
	}
	return -1
}
func (g *game) spellReason(a *avatar, i int) string {
	now := float64(g.Step) * .05
	if a.HP <= 0 {
		return "死亡中"
	}
	if a.Disconnected {
		return "切断中"
	}
	if a.Cast != nil {
		return "発動中"
	}
	if a.hasStatus("stun", now) {
		return "スタン中"
	}
	if a.spellIDs[i] == "flash" && a.hasStatus("root", now) {
		return "ルート中"
	}
	if a.spellCD[i] > now+1e-8 {
		return "クールダウン中"
	}
	return ""
}
func (g *game) spellViews(a *avatar) []skillView {
	views := []skillView{}
	for i, id := range a.spellIDs {
		d, ok := spellDefinitions[id]
		if !ok {
			continue
		}
		d.Slot = []string{"d", "f"}[i]
		amount := 0.0
		if id == "ignite" {
			amount = 50
		}
		if id == "barrier" {
			amount = 100
		}
		views = append(views, skillView{skillDefinition: d, Rank: 1, Amount: amount, ReadyAt: a.spellCD[i], Reason: g.spellReason(a, i)})
	}
	return views
}
func (g *game) castSpell(a *avatar, cmd command) string {
	i := spellIndex(cmd.Slot)
	if i < 0 {
		return "スペルが不正です"
	}
	id := a.spellIDs[i]
	d, ok := spellDefinitions[id]
	if !ok {
		return "未選択のスペルです"
	}
	if reason := g.spellReason(a, i); reason != "" {
		return reason
	}
	now := float64(g.Step) * .05
	switch id {
	case "flash":
		if !validPoint(cmd.Position) {
			return "座標が不正です"
		}
		dist := distance(a.Position, *cmd.Position)
		if dist < 1e-8 {
			return "移動距離不足"
		}
		ratio := math.Min(1, d.Range/dist)
		end := point{a.Position.S + (cmd.Position.S-a.Position.S)*ratio, a.Position.T + (cmd.Position.T-a.Position.T)*ratio}
		end = blinkDestination(a.Position, end)
		if distance(a.Position, end) < 1 {
			return "移動距離不足"
		}
		a.stop()
		a.Position = end
	case "ignite":
		target := g.actor(cmd.Target)
		if target == nil || target.Team == a.Team || target.HP <= 0 || !g.visible(a, target) {
			return "対象を指定できません"
		}
		if distance(a.Position, target.Position) > d.Range {
			return "射程外"
		}
		a.RecallUntil = 0
		target.addStatus(statusEffect{ID: "ignite", Source: a.ID, Kind: "ignite", Until: now + 5, nextTick: now + 1, interval: 1, damage: 10, trueDamage: true})
		if bushAt(a.Position) >= 0 {
			a.revealedUntil = now + 2
		}
	case "barrier":
		a.RecallUntil = 0
		a.addStatus(statusEffect{ID: "barrier", Source: a.ID, Kind: "shield", Until: now + 5, Value: 100})
	}
	a.spellCD[i] = now + d.Cooldown
	return ""
}

// Return the furthest legal point on the segment. Only landing occupancy matters:
// a legal endpoint beyond a building is reachable without testing the path.
func blinkDestination(start, end point) point {
	d := distance(start, end)
	if d < 1e-8 {
		return start
	}
	dir := point{(end.S - start.S) / d, (end.T - start.T) / d}
	limit := d
	if dir.S > 0 {
		limit = math.Min(limit, (arena.Length-arena.Radius-start.S)/dir.S)
	} else if dir.S < 0 {
		limit = math.Min(limit, (arena.Radius-start.S)/dir.S)
	}
	if dir.T > 0 {
		limit = math.Min(limit, (arena.Width/2-arena.Radius-start.T)/dir.T)
	} else if dir.T < 0 {
		limit = math.Min(limit, (-arena.Width/2+arena.Radius-start.T)/dir.T)
	}
	for tries := 0; tries <= len(arena.Structures); tries++ {
		p := point{start.S + dir.S*math.Max(0, limit), start.T + dir.T*math.Max(0, limit)}
		if legal(p) {
			return p
		}
		changed := false
		for _, o := range arena.Structures {
			if distance(p, o.Position) < o.Radius+arena.Radius+.01 {
				entry := rayCircle(start, dir, o.Position, o.Radius+arena.Radius+.010001)
				if entry <= limit {
					limit = math.Max(0, entry-.000001)
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	return start
}
