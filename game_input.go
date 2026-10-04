package main

func (l *lobby) gameInput(c *client, cmd command) {
	result := func(reason string) {
		l.emit(c, map[string]any{"type": "commandResult", "sequence": cmd.Sequence, "ok": reason == "", "reason": reason})
	}
	s := c.session
	m := l.matches[s.matchID]
	if m == nil || m.ID != cmd.MatchID || m.Phase != "playing" || m.Game == nil || m.Game.Winner != "" {
		result("試合が一致しません")
		return
	}
	a := m.Game.actor(s.id)
	if a == nil {
		return
	}
	if cmd.Sequence <= a.sequence {
		result("古い入力です")
		return
	}
	a.sequence = cmd.Sequence
	now := float64(m.Game.Step) * .05
	if a.HP <= 0 {
		result("死亡中")
		return
	}
	if cmd.Type == "upgrade" {
		result(m.Game.upgrade(a, cmd.Slot))
		return
	}
	if a.Cast != nil {
		result("発動中は操作できません")
		return
	}
	if cmd.Type == "cast" {
		result(m.Game.startCast(a, cmd))
		return
	}
	if cmd.Type == "spell" {
		result(m.Game.castSpell(a, cmd))
		return
	}
	if a.hasStatus("stun", now) {
		result("スタン中")
		return
	}
	switch cmd.Type {
	case "move":
		if a.hasStatus("root", now) {
			result("ルート中")
			return
		}
		if !validPoint(cmd.Position) {
			result("座標が不正です")
			return
		}
		a.stop()
		a.path = m.Game.navigation(a).findPath(a.Position, *cmd.Position)
	case "stop":
		a.stop()
	case "attack":
		target := m.Game.actor(cmd.Target)
		if target == nil || (target.isStructure() && !target.unlocked) || target.Team == a.Team || target.HP <= 0 || !m.Game.visible(a, target) {
			result("対象を指定できません")
			return
		}
		a.stop()
		a.attackTarget = target.ID
	case "recall":
		if a.hasStatus("root", now) || a.hasStatus("silence", now) {
			result("状態異常中はリコールできません")
			return
		}
		a.stop()
		a.RecallUntil = now + 8
	default:
		result("未対応の操作です")
		return
	}
	a.Moving = len(a.path) > 0
	result("")
}
