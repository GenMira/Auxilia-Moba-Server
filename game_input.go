package main

import "math"

func (l *lobby) gameInput(c *client, cmd command) {
	s := c.session
	m := l.matches[s.matchID]
	if m == nil || m.ID != cmd.MatchID || m.Phase != "playing" || m.Game == nil {
		return
	}
	for _, a := range m.Game.Actors {
		if a.ID != s.id || a.HP <= 0 {
			continue
		}
		if cmd.Sequence <= a.sequence {
			return
		}
		a.sequence = cmd.Sequence
		switch cmd.Type {
		case "move":
			if cmd.Position == nil || math.IsNaN(cmd.Position.S) || math.IsNaN(cmd.Position.T) || math.IsInf(cmd.Position.S, 0) || math.IsInf(cmd.Position.T, 0) || math.Abs(cmd.Position.S) > 1e6 || math.Abs(cmd.Position.T) > 1e6 {
				return
			}
			a.stop()
			a.path = findPath(a.Position, *cmd.Position)
		case "stop":
			a.stop()
		case "attack":
			target := m.Game.actor(cmd.Target)
			if target == nil || target.ID == a.ID || target.HP <= 0 || !m.Game.visible(a, target) {
				return
			}
			a.stop()
			a.attackTarget = target.ID
		case "recall":
			a.stop()
			a.RecallUntil = float64(m.Game.Step)*.05 + 8
		}
		a.Moving = len(a.path) > 0
	}

}
