package main

import "math"

// Definitions are also sent to the owner: aiming and HUD use the server's values.
type skillDefinition struct {
	BlinkDistance                               float64 `json:"blinkDistance"`
	HitRange                                    float64 `json:"hitRange"`
	blinkDelay                                  float64
	poison                                      bool
	Slot                                        string  `json:"slot"`
	Name                                        string  `json:"name"`
	Aim                                         string  `json:"aim"`
	Shape                                       string  `json:"shape"`
	Range                                       float64 `json:"range"`
	Radius                                      float64 `json:"radius"`
	Width                                       float64 `json:"width"`
	Angle                                       float64 `json:"angle"`
	Mana                                        float64 `json:"mana"`
	Cooldown                                    float64 `json:"cooldown"`
	Duration                                    float64 `json:"duration"`
	Description                                 string  `json:"description"`
	base                                        [3]float64
	attackRatio, powerRatio, slow, slowDuration float64
}

var skillDefinitions = map[string][3]skillDefinition{
	"Nadia": {
		{Slot: "q", Name: "対処番号05：対包囲戦術", Aim: "self", Shape: "circle", Radius: 220, Mana: 30, Cooldown: 5, Duration: .2, base: [3]float64{50, 75, 100}, attackRatio: .5, poison: true, Description: "自身の周囲にダメージと毒。毒は毎秒8、3秒間。"},
		{Slot: "w", Name: "対処番号03：前方範囲殲滅", Aim: "direction", Shape: "cone", Range: 300, Angle: 100, Mana: 45, Cooldown: 8, Duration: .3, base: [3]float64{70, 100, 130}, attackRatio: .7, poison: true, Description: "前方100度の扇形にダメージと毒。"},
		{Slot: "e", Name: "対処番号02：前方殲滅・改", Aim: "target", Shape: "target", Range: 480, HitRange: 180, BlinkDistance: 300, blinkDelay: .2, Mana: 60, Cooldown: 13, Duration: .5, base: [3]float64{100, 140, 180}, attackRatio: .9, poison: true, Description: "敵へ最大300Uブリンクし、到着後180U以内の指定対象にダメージと毒。"},
	},
	"Chiyo": {
		{Slot: "q", Name: "一文字斬り", Aim: "direction", Shape: "cone", Range: 300, Angle: 180, Mana: 30, Cooldown: 6, Duration: .2, base: [3]float64{60, 90, 120}, attackRatio: .8, Description: "前方180度の扇形にダメージ。HP60%以上で開始すると威力+10%。"},
		{Slot: "w", Name: "袈裟斬り", Aim: "direction", Shape: "cone", Range: 200, Angle: 90, BlinkDistance: 100, blinkDelay: .1, Mana: 45, Cooldown: 9, Duration: .4, base: [3]float64{80, 120, 160}, attackRatio: .8, Description: "100Uブリンク後、到着地点から90度の扇形攻撃。HP60%以上で開始すると威力+10%。"},
		{Slot: "e", Name: "真向斬り", Aim: "target", Shape: "target", Range: 200, Mana: 60, Cooldown: 15, Duration: .3, base: [3]float64{100, 140, 180}, attackRatio: 1.1, Description: "可視の敵1体にダメージ。発動完了時も射程200U・視界が必要。"},
	},
	"Sophie": {
		{Slot: "q", Name: "growth～成長～", Aim: "direction", Shape: "line", Range: 600, Width: 100, Mana: 40, Cooldown: 6, Duration: .2, base: [3]float64{60, 90, 120}, attackRatio: .6, Description: "直線上の敵すべてにダメージ。施設で遮られる。"},
		{Slot: "w", Name: "bloom～開花～", Aim: "point", Shape: "circle", Range: 700, Radius: 180, Mana: 60, Cooldown: 10, Duration: .4, base: [3]float64{70, 100, 130}, powerRatio: .6, slow: .2, slowDuration: 2, Description: "指定円内にダメージと20%スロウ（2秒）。"},
		{Slot: "e", Name: "fruit～結実～", Aim: "direction", Shape: "projectile", Range: 900, Width: 70, Mana: 90, Cooldown: 18, Duration: .6, base: [3]float64{120, 170, 220}, attackRatio: 1, Description: "速度1600U/sの弾。最初の敵に命中し、施設・境界で消滅。"},
	},
	"Jude": {
		{Slot: "q", Name: "急襲", Aim: "direction", Shape: "cone", Range: 250, Angle: 90, Mana: 30, Cooldown: 6, Duration: .2, base: [3]float64{60, 90, 120}, attackRatio: .6, slow: .25, slowDuration: 3, Description: "前方90度の扇形にダメージと25%スロウ（3秒）。"},
		{Slot: "w", Name: "切り裂き", Aim: "self", Shape: "circle", Radius: 220, Mana: 45, Cooldown: 9, Duration: .3, base: [3]float64{80, 120, 160}, attackRatio: .8, Description: "自身を中心とする円内の敵にダメージ。"},
		{Slot: "e", Name: "応急手当", Aim: "self", Shape: "heal", Mana: 60, Cooldown: 16, Duration: .3, base: [3]float64{70, 110, 150}, powerRatio: .4, Description: "自身のHPを回復し、毒・スロウを解除。イグナイトは解除しない。"},
	},
}

type skillView struct {
	UpgradeReason string `json:"upgradeReason"`
	skillDefinition
	Rank    int     `json:"rank"`
	Amount  float64 `json:"amount"`
	ReadyAt float64 `json:"readyAt"`
	Reason  string  `json:"reason"`
}
type castState struct {
	blinkAt       float64
	blinked       bool
	outgoingBonus float64
	Slot          string  `json:"slot"`
	Shape         string  `json:"shape"`
	Origin        point   `json:"origin"`
	Direction     point   `json:"direction"`
	Destination   point   `json:"destination"`
	EndsAt        float64 `json:"endsAt"`
	definition    skillDefinition
	amount        float64
	target        string
}
type skillEffect struct {
	ID        uint64  `json:"id"`
	Owner     string  `json:"owner"`
	Shape     string  `json:"shape"`
	Origin    point   `json:"origin"`
	Direction point   `json:"direction"`
	Range     float64 `json:"range"`
	Radius    float64 `json:"radius"`
	Width     float64 `json:"width"`
	Angle     float64 `json:"angle"`
	Until     float64 `json:"until"`
}

func slotIndex(slot string) int {
	switch slot {
	case "q":
		return 0
	case "w":
		return 1
	case "e":
		return 2
	}
	return -1
}
func validPoint(p *point) bool {
	return p != nil && !math.IsNaN(p.S) && !math.IsNaN(p.T) && !math.IsInf(p.S, 0) && !math.IsInf(p.T, 0) && math.Abs(p.S) <= 1e6 && math.Abs(p.T) <= 1e6
}
func (g *game) skillReason(a *avatar, d skillDefinition, i int) string {
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
	if a.hasStatus("silence", now) {
		return "サイレンス中"
	}
	if d.BlinkDistance > 0 && a.hasStatus("root", now) {
		return "ルート中"
	}
	if a.cooldowns[i] > now+1e-8 {
		return "クールダウン中"
	}
	if a.Mana < d.Mana {
		return "マナ不足"
	}
	return ""
}
func (g *game) skillViews(a *avatar) []skillView {
	defs, ok := skillDefinitions[a.Character]
	if !ok {
		return nil
	}
	views := []skillView{}
	for i, d := range defs {
		rank := max(1, min(3, a.ranks[i]))
		amount := (d.base[rank-1] + a.Stats.Attack*d.attackRatio + a.SkillPower*d.powerRatio) * (1 + a.chiyoBonus(d.Slot))
		views = append(views, skillView{skillDefinition: d, Rank: rank, Amount: amount, ReadyAt: a.cooldowns[i], Reason: g.skillReason(a, d, i), UpgradeReason: upgradeReason(a, i)})
	}
	return views
}
func (g *game) startCast(a *avatar, cmd command) string {
	i := slotIndex(cmd.Slot)
	defs, ok := skillDefinitions[a.Character]
	if i < 0 || !ok {
		return "未実装のスキルです"
	}
	d := defs[i]
	if reason := g.skillReason(a, d, i); reason != "" {
		return reason
	}
	dest, dir := a.Position, a.Facing
	if d.Aim == "target" {
		target := g.actor(cmd.Target)
		if target == nil || target.Team == a.Team || target.HP <= 0 || !g.visible(a, target) {
			return "対象を指定できません"
		}
		if distance(a.Position, target.Position) > d.Range {
			return "射程外"
		}
		dest = target.Position
	} else if d.Aim == "direction" || d.Aim == "point" {
		if !validPoint(cmd.Position) {
			return "座標が不正です"
		}
		dest = *cmd.Position
	}
	distanceTo := distance(a.Position, dest)
	if distanceTo > 1e-8 {
		dir = point{(dest.S - a.Position.S) / distanceTo, (dest.T - a.Position.T) / distanceTo}
	}
	if d.Aim == "point" && distanceTo > d.Range {
		dest = point{a.Position.S + dir.S*d.Range, a.Position.T + dir.T*d.Range}
	}
	now := float64(g.Step) * .05
	rank := max(1, min(3, a.ranks[i]))
	a.stop()
	a.Facing = dir
	a.Mana -= d.Mana
	a.cooldowns[i] = now + d.Cooldown
	a.Cast = &castState{Slot: cmd.Slot, Shape: d.Shape, Origin: a.Position, Direction: dir, Destination: dest, EndsAt: now + d.Duration, definition: d, amount: d.base[rank-1] + a.Stats.Attack*d.attackRatio + a.SkillPower*d.powerRatio, target: cmd.Target}
	a.Cast.outgoingBonus = a.chiyoBonus(cmd.Slot)
	if d.BlinkDistance > 0 {
		travel := d.BlinkDistance
		if d.Aim == "target" {
			travel = math.Min(travel, math.Max(0, distanceTo-150))
		}
		a.Cast.Destination = blinkDestination(a.Position, point{a.Position.S + dir.S*travel, a.Position.T + dir.T*travel})
		a.Cast.blinkAt = now + d.blinkDelay
	}
	if d.Shape != "heal" && bushAt(a.Position) >= 0 {
		a.revealedUntil = now + 2
	}
	return ""
}
func (g *game) interruptCasts(now float64) {
	for _, a := range g.Actors {
		if a.Cast != nil && a.Cast.definition.BlinkDistance > 0 && !a.Cast.blinked && a.hasStatus("root", now) {
			a.Cast = nil
		}
		if a.HP <= 0 || a.hasStatus("stun", now) || a.hasStatus("silence", now) {
			a.Cast = nil
		}
		if a.hasStatus("stun", now) {
			a.stop()
		}
		if a.hasStatus("stun", now) || a.hasStatus("silence", now) || a.hasStatus("root", now) {
			a.RecallUntil = 0
		}
	}
}

// All due heals resolve before the common damage batch, including mutual kills.
func (g *game) resolveSkills() []hit {
	now := float64(g.Step) * .05
	hits := []hit{}
	kept := g.Effects[:0]
	for _, e := range g.Effects {
		if e.Until > now {
			kept = append(kept, e)
		}
	}
	g.Effects = kept
	// Resolve every blink before any same-tick area/target hit tests.
	for _, a := range g.Actors {
		c := a.Cast
		if c != nil && c.definition.BlinkDistance > 0 && !c.blinked && c.blinkAt <= now+1e-8 {
			a.Position = blinkDestination(a.Position, c.Destination)
			c.Origin = a.Position
			c.blinked = true
		}
	}
	for _, a := range g.Actors {
		c := a.Cast
		if c == nil || c.EndsAt > now+1e-8 || a.HP <= 0 {
			continue
		}
		a.Cast = nil
		d := c.definition
		if d.Shape == "heal" {
			a.HP = math.Min(a.Stats.HP, a.HP+c.amount)
			a.cleanse("poison", "slow")
		}
		if d.Shape == "projectile" {
			g.projectileID++
			g.Projectiles = append(g.Projectiles, projectile{ID: g.projectileID, Owner: a.ID, Position: c.Origin, Kind: "skill", Direction: c.Direction, Remaining: d.Range, Width: d.Width, damage: c.amount, spawnStep: g.Step})
			continue
		}
		origin := c.Origin
		if d.Aim == "point" {
			origin = c.Destination
		}
		length := d.Range
		if d.Shape == "line" {
			length = obstacleDistance(origin, c.Direction, length, d.Width/2)
		}
		g.eventID++
		g.Effects = append(g.Effects, skillEffect{g.eventID, a.ID, d.Shape, origin, c.Direction, length, d.Radius, d.Width, d.Angle, now + .35})
		for _, target := range g.Actors {
			if target.Team == a.Team || target.HP <= 0 || d.Shape == "heal" {
				continue
			}
			hitTarget := false
			switch d.Shape {
			case "circle":
				hitTarget = distance(origin, target.Position) <= d.Radius+arena.Radius
			case "cone":
				hitTarget = coneIntersects(origin, c.Direction, d.Range, d.Angle, target.Position, arena.Radius)
			case "line":
				hitTarget = segmentDistance(origin, point{origin.S + c.Direction.S*length, origin.T + c.Direction.T*length}, target.Position) <= d.Width/2+arena.Radius
			case "target":
				r := d.Range
				if d.HitRange > 0 {
					r = d.HitRange
				}
				hitTarget = target.ID == c.target && g.visible(a, target) && distance(a.Position, target.Position) <= r
			}
			if hitTarget {
				h := g.newHit(a, target, c.amount, "skill", c.EndsAt)
				h.outgoingBonus = c.outgoingBonus
				h.poison = d.poison
				h.slow = d.slow
				h.slowDuration = d.slowDuration
				h.effectID = a.Character + "-" + d.Slot
				hits = append(hits, h)
			}
		}
	}
	return hits
}

// Sector/circle intersection includes radial edges, the arc and the sector origin.
func coneIntersects(origin, dir point, radius, angle float64, target point, targetRadius float64) bool {
	x, y := target.S-origin.S, target.T-origin.T
	dist := math.Hypot(x, y)
	if dist <= targetRadius {
		return true
	}
	half := angle * math.Pi / 360
	dot := (x*dir.S + y*dir.T) / dist
	if dot >= math.Cos(half) && dist <= radius+targetRadius {
		return true
	}
	for _, sign := range []float64{-1, 1} {
		c, s := math.Cos(half*sign), math.Sin(half*sign)
		end := point{origin.S + radius*(dir.S*c-dir.T*s), origin.T + radius*(dir.S*s+dir.T*c)}
		if segmentDistance(origin, end, target) <= targetRadius {
			return true
		}
	}
	return false
}

// Swept disk collision; returns the first contact distance (including tangent contact).
func rayCircle(origin, dir, center point, radius float64) float64 {
	x, y := origin.S-center.S, origin.T-center.T
	c := x*x + y*y - radius*radius
	if c <= 0 {
		return 0
	}
	b := x*dir.S + y*dir.T
	disc := b*b - c
	if disc < 0 {
		return math.Inf(1)
	}
	t := -b - math.Sqrt(disc)
	if t < 0 {
		return math.Inf(1)
	}
	return t
}
func obstacleDistance(origin, dir point, limit, width float64) float64 {
	for _, o := range arena.Structures {
		limit = math.Min(limit, rayCircle(origin, dir, o.Position, o.Radius+width))
	}
	// Border crossing is based on the projectile center, matching the map rule.
	if dir.S > 0 {
		limit = math.Min(limit, (arena.Length-origin.S)/dir.S)
	} else if dir.S < 0 {
		limit = math.Min(limit, -origin.S/dir.S)
	}
	if dir.T > 0 {
		limit = math.Min(limit, (arena.Width/2-origin.T)/dir.T)
	} else if dir.T < 0 {
		limit = math.Min(limit, (-arena.Width/2-origin.T)/dir.T)
	}
	return math.Max(0, limit)
}
