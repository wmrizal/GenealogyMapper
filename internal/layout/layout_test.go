package layout

import (
	"testing"

	"genealogymapper/internal/domain"
)

func TestComputePlacesAncestorsAboveAndChildrenBelow(t *testing.T) {
	graph := domain.GraphSnapshot{
		People: []domain.Person{
			{ID: "parent", Name: "Parent"},
			{ID: "root", Name: "Root"},
			{ID: "child", Name: "Child"},
		},
		ParentChild: []domain.ParentChild{
			{ParentID: "parent", ChildID: "root"},
			{ParentID: "root", ChildID: "child"},
		},
	}

	snapshot := Compute(graph, "root")
	parent := snapshot.Nodes["parent"]
	root := snapshot.Nodes["root"]
	child := snapshot.Nodes["child"]
	if parent.Center.Y >= root.Center.Y {
		t.Fatalf("parent Y = %v, root Y = %v", parent.Center.Y, root.Center.Y)
	}
	if child.Center.Y <= root.Center.Y {
		t.Fatalf("child Y = %v, root Y = %v", child.Center.Y, root.Center.Y)
	}
	if !root.Bounds.Contains(root.Center) {
		t.Fatal("root center should be inside its node bounds")
	}
}

func TestComputePlacesSiblingsOnSelectedLevel(t *testing.T) {
	graph := domain.GraphSnapshot{
		People: []domain.Person{
			{ID: "parent", Name: "Parent"},
			{ID: "root", Name: "Root"},
			{ID: "sibling", Name: "Sibling"},
		},
		ParentChild: []domain.ParentChild{
			{ParentID: "parent", ChildID: "root"},
			{ParentID: "parent", ChildID: "sibling"},
		},
	}

	snapshot := Compute(graph, "root")
	if snapshot.Nodes["sibling"].Center.Y != snapshot.Nodes["root"].Center.Y {
		t.Fatalf("sibling Y = %v, root Y = %v", snapshot.Nodes["sibling"].Center.Y, snapshot.Nodes["root"].Center.Y)
	}
	if snapshot.Nodes["sibling"].Center.X == snapshot.Nodes["root"].Center.X {
		t.Fatal("sibling and root should not overlap")
	}
}

func countTestCrossings(snapshot Snapshot) int {
	total := 0
	rels := snapshot.ParentChild
	for i := range rels {
		for j := i + 1; j < len(rels); j++ {
			p1, c1 := snapshot.Nodes[rels[i].ParentID].Center, snapshot.Nodes[rels[i].ChildID].Center
			p2, c2 := snapshot.Nodes[rels[j].ParentID].Center, snapshot.Nodes[rels[j].ChildID].Center
			if p1.Y == p2.Y && c1.Y == c2.Y && (p1.X-p2.X)*(c1.X-c2.X) < 0 {
				total++
			}
		}
	}
	return total
}

func TestComputeAvoidsCrossingsAndIsDeterministic(t *testing.T) {
	graph := domain.GraphSnapshot{
		People: []domain.Person{
			{ID: "p1", Name: "A"}, {ID: "p2", Name: "B"},
			{ID: "c1", Name: "A child of B"}, {ID: "c2", Name: "B child of A"},
		},
		ParentChild: []domain.ParentChild{
			{ParentID: "p1", ChildID: "c2"},
			{ParentID: "p2", ChildID: "c1"},
		},
	}

	first := ComputeFull(graph, "")
	if got := countTestCrossings(first); got != 0 {
		t.Fatalf("crossings = %d, want 0", got)
	}
	second := ComputeFull(graph, "")
	for id, node := range first.Nodes {
		if second.Nodes[id].Center != node.Center {
			t.Fatalf("layout not deterministic for %s", id)
		}
	}
}

func TestComputePlacesCoParentWithPartnerNotSelected(t *testing.T) {
	graph := domain.GraphSnapshot{
		People: []domain.Person{
			{ID: "bob", Name: "Bob"}, {ID: "dave", Name: "Dave"},
			{ID: "jane", Name: "Jane"}, {ID: "frank", Name: "Frank"},
		},
		ParentChild: []domain.ParentChild{
			{ParentID: "bob", ChildID: "dave"},
			{ParentID: "dave", ChildID: "frank"},
			{ParentID: "jane", ChildID: "frank"},
		},
	}

	snapshot := Compute(graph, "bob")
	if snapshot.Nodes["jane"].Center.Y != snapshot.Nodes["dave"].Center.Y {
		t.Fatalf("jane Y = %v, dave Y = %v", snapshot.Nodes["jane"].Center.Y, snapshot.Nodes["dave"].Center.Y)
	}
}

func TestViewportZoomKeepsCursorWorldPointStable(t *testing.T) {
	viewport := Viewport{Scale: 1, Offset: Point{X: 20, Y: 40}}
	cursor := Point{X: 140, Y: 180}
	worldBefore := viewport.World(cursor)
	zoomed := viewport.ZoomAt(cursor, 2)
	worldAfter := zoomed.World(cursor)

	if worldBefore != worldAfter {
		t.Fatalf("world point changed from %#v to %#v", worldBefore, worldAfter)
	}
	if zoomed.Scale != 2 {
		t.Fatalf("zoom scale = %v, want 2", zoomed.Scale)
	}
}
