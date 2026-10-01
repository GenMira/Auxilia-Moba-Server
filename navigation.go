package main

import (
	"container/heap"
	"math"
)

func distance(a, b point) float64 { return math.Hypot(a.S-b.S, a.T-b.T) }
func legal(p point) bool {
	if p.S < arena.Radius || p.S > arena.Length-arena.Radius || p.T < -arena.Width/2+arena.Radius || p.T > arena.Width/2-arena.Radius {
		return false
	}
	for _, o := range arena.Structures {
		if distance(p, o.Position) < o.Radius+arena.Radius+0.01 {
			return false
		}
	}
	return true
}
func clearSegment(a, b point) bool {
	if !legal(a) || !legal(b) {
		return false
	}
	dx, dy := b.S-a.S, b.T-a.T
	square := dx*dx + dy*dy
	for _, o := range arena.Structures {
		t := 0.0
		if square > 0 {
			t = math.Max(0, math.Min(1, ((o.Position.S-a.S)*dx+(o.Position.T-a.T)*dy)/square))
		}
		if distance(point{a.S + t*dx, a.T + t*dy}, o.Position) < o.Radius+arena.Radius+0.01 {
			return false
		}
	}
	return true
}
func nodePoint(id int) point { return point{float64(id/29) * 50, float64(id%29)*50 - 700} }
func nearestNode(p point, connect bool) int {
	best := -1
	length := math.Inf(1)
	for id := 0; id < 121*29; id++ {
		q := nodePoint(id)
		if !legal(q) || (connect && !clearSegment(p, q)) {
			continue
		}
		d := distance(p, q)
		if d < length {
			best = id
			length = d
		}
	}
	return best
}

type searchNode struct {
	id   int
	cost float64
}
type nodeHeap []searchNode

func (h nodeHeap) Len() int { return len(h) }
func (h nodeHeap) Less(i, j int) bool {
	if h[i].cost == h[j].cost {
		return h[i].id < h[j].id
	}
	return h[i].cost < h[j].cost
}
func (h nodeHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *nodeHeap) Push(v any)   { *h = append(*h, v.(searchNode)) }
func (h *nodeHeap) Pop() any     { a := *h; v := a[len(a)-1]; *h = a[:len(a)-1]; return v }
func findPath(start, target point) []point {
	if !legal(target) {
		n := nearestNode(target, false)
		if n < 0 {
			return nil
		}
		target = nodePoint(n)
	}
	if clearSegment(start, target) {
		return []point{target}
	}
	first, last := nearestNode(start, true), nearestNode(target, true)
	if first < 0 || last < 0 {
		return nil
	}
	costs := map[int]float64{first: 0}
	parent := map[int]int{}
	closed := map[int]bool{}
	open := &nodeHeap{{first, distance(nodePoint(first), target)}}
	heap.Init(open)
	for open.Len() > 0 {
		current := heap.Pop(open).(searchNode).id
		if closed[current] {
			continue
		}
		closed[current] = true
		if current == last {
			reversed := []point{target}
			for n := last; ; n = parent[n] {
				reversed = append(reversed, nodePoint(n))
				if n == first {
					break
				}
			}
			route := []point{start}
			for i := len(reversed) - 1; i >= 0; i-- {
				route = append(route, reversed[i])
			}
			result := []point{}
			for i := 0; i < len(route)-1; {
				j := len(route) - 1
				for j > i+1 && !clearSegment(route[i], route[j]) {
					j--
				}
				result = append(result, route[j])
				i = j
			}
			return result
		}
		x, y := current/29, current%29
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				if dx == 0 && dy == 0 {
					continue
				}
				nx, ny := x+dx, y+dy
				if nx < 0 || nx > 120 || ny < 0 || ny > 28 {
					continue
				}
				next := nx*29 + ny
				if closed[next] || !clearSegment(nodePoint(current), nodePoint(next)) {
					continue
				}
				cost := costs[current] + distance(nodePoint(current), nodePoint(next))
				old, ok := costs[next]
				if !ok || cost < old {
					costs[next] = cost
					parent[next] = current
					heap.Push(open, searchNode{next, cost + distance(nodePoint(next), target)})
				}
			}
		}
	}
	return nil
}
func bushAt(p point) int {
	for i, b := range arena.Bushes {
		if p.S >= b.MinS && p.S <= b.MaxS && p.T >= b.MinT && p.T <= b.MaxT {
			return i
		}
	}
	return -1
}
