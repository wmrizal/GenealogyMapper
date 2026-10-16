package layout

import (
	"sort"

	"genealogymapper/internal/domain"
)

const (
	NodeWidth     = 180.0
	NodeHeight    = 72.0
	HorizontalGap = 42.0
	VerticalGap   = 124.0
	MinZoom       = 0.35
	MaxZoom       = 2.50
	DefaultZoom   = 1.0
)

type Point struct {
	X float64
	Y float64
}

type Rect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

func (r Rect) Contains(point Point) bool {
	return point.X >= r.X && point.X <= r.X+r.Width && point.Y >= r.Y && point.Y <= r.Y+r.Height
}

type Node struct {
	Person domain.Person
	Center Point
	Bounds Rect
}

type Snapshot struct {
	Nodes       map[string]Node
	ParentChild []domain.ParentChild
	SelectedID  string
}

// Compute lays out the graph around selectedID.
func Compute(graph domain.GraphSnapshot, selectedID string) Snapshot {
	return compute(graph, selectedID, false)
}

// ComputeFull lays out every person by generation, with roots on level 0.
func ComputeFull(graph domain.GraphSnapshot, selectedID string) Snapshot {
	return compute(graph, selectedID, true)
}

func generationLevels(peopleByID map[string]domain.Person, parentsByChild map[string][]string) map[string]int {
	levels := make(map[string]int, len(peopleByID))
	visiting := make(map[string]bool)
	var depth func(id string) int
	depth = func(id string) int {
		if level, done := levels[id]; done {
			return level
		}
		if visiting[id] {
			return 0
		}
		visiting[id] = true
		level := 0
		for _, parentID := range parentsByChild[id] {
			if parentLevel := depth(parentID) + 1; parentLevel > level {
				level = parentLevel
			}
		}
		visiting[id] = false
		levels[id] = level
		return level
	}
	for id := range peopleByID {
		depth(id)
	}
	return levels
}

func compute(graph domain.GraphSnapshot, selectedID string, full bool) Snapshot {
	peopleByID := make(map[string]domain.Person, len(graph.People))
	parentsByChild := make(map[string][]string)
	childrenByParent := make(map[string][]string)
	for _, person := range graph.People {
		peopleByID[person.ID] = person
	}
	for _, relation := range graph.ParentChild {
		if _, parentExists := peopleByID[relation.ParentID]; !parentExists {
			continue
		}
		if _, childExists := peopleByID[relation.ChildID]; !childExists {
			continue
		}
		parentsByChild[relation.ChildID] = append(parentsByChild[relation.ChildID], relation.ParentID)
		childrenByParent[relation.ParentID] = append(childrenByParent[relation.ParentID], relation.ChildID)
	}
	if _, selectedExists := peopleByID[selectedID]; !selectedExists {
		selectedID = ""
	}

	levels := make(map[string]int, len(peopleByID))
	if full {
		levels = generationLevels(peopleByID, parentsByChild)
	} else if selectedID != "" {
		levels[selectedID] = 0
		collectLevels(parentsByChild, selectedID, -1, levels)
		collectLevels(childrenByParent, selectedID, 1, levels)
		for _, parentID := range parentsByChild[selectedID] {
			for _, siblingID := range childrenByParent[parentID] {
				if siblingID != selectedID {
					if _, assigned := levels[siblingID]; !assigned {
						levels[siblingID] = 0
					}
				}
			}
		}
		// Place remaining relatives (co-parents, cousins) relative to anyone already placed.
		for changed := true; changed; {
			changed = false
			for _, person := range graph.People {
				if _, assigned := levels[person.ID]; assigned {
					continue
				}
				if level, ok := levelFromNeighbors(levels, childrenByParent[person.ID], -1, minInt); ok {
					levels[person.ID], changed = level, true
				} else if level, ok := levelFromNeighbors(levels, parentsByChild[person.ID], 1, maxInt); ok {
					levels[person.ID], changed = level, true
				}
			}
		}
	}
	for id := range peopleByID {
		if _, assigned := levels[id]; !assigned {
			levels[id] = 0
		}
	}

	idsByLevel := make(map[int][]string)
	for id, level := range levels {
		idsByLevel[level] = append(idsByLevel[level], id)
	}
	for _, ids := range idsByLevel {
		sort.Slice(ids, func(i, j int) bool {
			left := peopleByID[ids[i]]
			right := peopleByID[ids[j]]
			if left.Name == right.Name {
				return left.ID < right.ID
			}
			return left.Name < right.Name
		})
	}

	orderLevels(idsByLevel, parentsByChild, childrenByParent)

	nodes := make(map[string]Node, len(peopleByID))
	for level, ids := range idsByLevel {
		for index, id := range ids {
			center := Point{
				X: (float64(index) - float64(len(ids)-1)/2) * (NodeWidth + HorizontalGap),
				Y: float64(level) * (NodeHeight + VerticalGap),
			}
			nodes[id] = Node{
				Person: peopleByID[id],
				Center: center,
				Bounds: Rect{
					X:      center.X - NodeWidth/2,
					Y:      center.Y - NodeHeight/2,
					Width:  NodeWidth,
					Height: NodeHeight,
				},
			}
		}
	}

	relations := make([]domain.ParentChild, 0, len(graph.ParentChild))
	for _, relation := range graph.ParentChild {
		if _, parentExists := nodes[relation.ParentID]; parentExists {
			if _, childExists := nodes[relation.ChildID]; childExists {
				relations = append(relations, relation)
			}
		}
	}
	sort.Slice(relations, func(i, j int) bool {
		if relations[i].ParentID == relations[j].ParentID {
			return relations[i].ChildID < relations[j].ChildID
		}
		return relations[i].ParentID < relations[j].ParentID
	})
	return Snapshot{Nodes: nodes, ParentChild: relations, SelectedID: selectedID}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// levelFromNeighbors returns offset plus the picked level among placed neighbors.
func levelFromNeighbors(levels map[string]int, neighbors []string, offset int, pick func(a, b int) int) (int, bool) {
	result, found := 0, false
	for _, id := range neighbors {
		level, assigned := levels[id]
		if !assigned {
			continue
		}
		if !found {
			result, found = level, true
		} else {
			result = pick(result, level)
		}
	}
	return result + offset, found
}

// orderLevels reorders each level in place to reduce parent-child line crossings.
func orderLevels(idsByLevel map[int][]string, parents, children map[string][]string) {
	levels := make([]int, 0, len(idsByLevel))
	levelOf := make(map[string]int)
	for level, ids := range idsByLevel {
		levels = append(levels, level)
		for _, id := range ids {
			levelOf[id] = level
		}
	}
	sort.Ints(levels)
	if len(levels) < 2 {
		return
	}

	type edge struct{ parent, child string }
	edges := make([]edge, 0)
	for child, list := range parents {
		for _, parent := range list {
			if levelOf[parent] < levelOf[child] {
				edges = append(edges, edge{parent, child})
			}
		}
	}

	pos := make(map[string]float64, len(levelOf))
	refresh := func(level int) {
		ids := idsByLevel[level]
		for i, id := range ids {
			pos[id] = float64(i) - float64(len(ids)-1)/2
		}
	}
	for _, level := range levels {
		refresh(level)
	}

	// Only edges sharing the same pair of levels are compared; level-skipping edges are approximated.
	crossings := func() int {
		total := 0
		for i := 0; i < len(edges); i++ {
			for j := i + 1; j < len(edges); j++ {
				a, b := edges[i], edges[j]
				if levelOf[a.parent] != levelOf[b.parent] || levelOf[a.child] != levelOf[b.child] {
					continue
				}
				if (pos[a.parent]-pos[b.parent])*(pos[a.child]-pos[b.child]) < 0 {
					total++
				}
			}
		}
		return total
	}

	snapshot := func() map[int][]string {
		copyOf := make(map[int][]string, len(idsByLevel))
		for level, ids := range idsByLevel {
			copyOf[level] = append([]string(nil), ids...)
		}
		return copyOf
	}
	best := snapshot()
	bestCount := crossings()

	sweep := func(level int, neighbors map[string][]string) {
		ids := idsByLevel[level]
		keys := make(map[string]float64, len(ids))
		for _, id := range ids {
			sum, count := 0.0, 0
			for _, other := range neighbors[id] {
				if levelOf[other] != level {
					sum += pos[other]
					count++
				}
			}
			if count > 0 {
				keys[id] = sum / float64(count)
			} else {
				keys[id] = pos[id]
			}
		}
		sort.SliceStable(ids, func(i, j int) bool { return keys[ids[i]] < keys[ids[j]] })
		refresh(level)
	}

	transpose := func() {
		if len(edges) > 300 {
			return
		}
		for improved := true; improved; {
			improved = false
			for _, level := range levels {
				ids := idsByLevel[level]
				for i := 0; i+1 < len(ids); i++ {
					before := crossings()
					ids[i], ids[i+1] = ids[i+1], ids[i]
					refresh(level)
					if crossings() < before {
						improved = true
					} else {
						ids[i], ids[i+1] = ids[i+1], ids[i]
						refresh(level)
					}
				}
			}
		}
	}

	for round := 0; round < 24 && bestCount > 0; round++ {
		if round%2 == 0 {
			for _, level := range levels[1:] {
				sweep(level, parents)
			}
		} else {
			for i := len(levels) - 2; i >= 0; i-- {
				sweep(levels[i], children)
			}
		}
		transpose()
		if count := crossings(); count < bestCount {
			bestCount = count
			best = snapshot()
		}
	}
	for level, ids := range best {
		copy(idsByLevel[level], ids)
	}
}

func collectLevels(adjacency map[string][]string, startID string, direction int, levels map[string]int) {
	frontier := []string{startID}
	visited := map[string]bool{startID: true}
	depth := 1
	for len(frontier) > 0 {
		next := make([]string, 0)
		for _, currentID := range frontier {
			for _, connectedID := range adjacency[currentID] {
				if visited[connectedID] {
					continue
				}
				visited[connectedID] = true
				levels[connectedID] = depth * direction
				next = append(next, connectedID)
			}
		}
		frontier = next
		depth++
	}
}

type Viewport struct {
	Scale  float64
	Offset Point
}

func DefaultViewport() Viewport {
	return Viewport{Scale: DefaultZoom}
}

func (v Viewport) Screen(world Point) Point {
	return Point{X: world.X*v.Scale + v.Offset.X, Y: world.Y*v.Scale + v.Offset.Y}
}

func (v Viewport) World(screen Point) Point {
	if v.Scale == 0 {
		return Point{}
	}
	return Point{X: (screen.X - v.Offset.X) / v.Scale, Y: (screen.Y - v.Offset.Y) / v.Scale}
}

func (v Viewport) ZoomAt(screen Point, factor float64) Viewport {
	worldBefore := v.World(screen)
	v.Scale *= factor
	if v.Scale < MinZoom {
		v.Scale = MinZoom
	}
	if v.Scale > MaxZoom {
		v.Scale = MaxZoom
	}
	v.Offset = Point{X: screen.X - worldBefore.X*v.Scale, Y: screen.Y - worldBefore.Y*v.Scale}
	return v
}
