package main

// Authoritative map geometry and level-one character attributes.
type point struct {
	S float64 `json:"s"`
	T float64 `json:"t"`
}
type structure struct {
	ID       string  `json:"id"`
	Team     string  `json:"team"`
	Kind     string  `json:"kind"`
	Position point   `json:"position"`
	Radius   float64 `json:"radius"`
	HP       float64 `json:"hp"`
}
type bush struct {
	MinS float64 `json:"minS"`
	MaxS float64 `json:"maxS"`
	MinT float64 `json:"minT"`
	MaxT float64 `json:"maxT"`
}
type mapDefinition struct {
	Length     float64     `json:"length"`
	Width      float64     `json:"width"`
	Radius     float64     `json:"characterRadius"`
	Grid       float64     `json:"grid"`
	Structures []structure `json:"structures"`
	Bushes     []bush      `json:"bushes"`
}

var arena = mapDefinition{6000, 1400, 35, 50, []structure{
	{"blue-base", "blue", "base", point{400, 0}, 100, 1500}, {"blue-inner", "blue", "tower", point{1200, 0}, 75, 2000}, {"blue-outer", "blue", "tower", point{2200, 0}, 75, 2000},
	{"red-base", "red", "base", point{5600, 0}, 100, 1500}, {"red-inner", "red", "tower", point{4800, 0}, 75, 2000}, {"red-outer", "red", "tower", point{3800, 0}, 75, 2000},
}, []bush{{2500, 2900, 450, 650}, {3100, 3500, -650, -450}}}

type characterDefinition struct {
	HP          float64 `json:"hp"`
	Mana        float64 `json:"mana"`
	Speed       float64 `json:"speed"`
	Attack      float64 `json:"attack"`
	Range       float64 `json:"range"`
	AttackSpeed float64 `json:"attackSpeed"`
	HPRegen     float64 `json:"hpRegen"`
	ManaRegen   float64 `json:"manaRegen"`
}

var characterDefinitions = map[string]characterDefinition{
	"Sophie": {600, 330, 350, 50, 500, .8, 2, 3}, "Jude": {850, 280, 350, 50, 150, .7, 3, 3}, "Nadia": {650, 300, 450, 55, 200, .9, 2, 3}, "Chiyo": {750, 280, 350, 60, 200, .9, 5, 3},
}
