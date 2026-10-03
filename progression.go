package main

func (a *avatar) chiyoBonus(action string) float64 {
	if a.Character == "Chiyo" && a.HP >= a.Stats.HP*.6 && (action == "attack" || action == "q" || action == "w") {
		return .1
	}
	return 0
}
func upgradeReason(a *avatar, i int) string {
	if a.HP <= 0 {
		return "死亡中"
	}
	if a.Disconnected {
		return "切断中"
	}
	rank := max(1, a.ranks[i])
	if rank >= 3 {
		return "最大ランク"
	}
	if a.Level < rank*2+1 {
		return "必要レベル未到達"
	}
	if a.SkillPoints <= 0 {
		return "ポイント不足"
	}
	return ""
}
func (g *game) upgrade(a *avatar, slot string) string {
	i := slotIndex(slot)
	if i < 0 {
		return "スキルが不正です"
	}
	if reason := upgradeReason(a, i); reason != "" {
		return reason
	}
	a.ranks[i] = max(1, a.ranks[i]) + 1
	a.SkillPoints--
	return ""
}
