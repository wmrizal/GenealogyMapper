package ui

import (
	"testing"
	"time"

	"genealogymapper/internal/layout"
)

func newTestCanvas() *graphCanvas {
	return &graphCanvas{previousIDs: map[string]bool{}, fadeStart: map[string]time.Time{}}
}

func snapshotOf(selected string, ids ...string) layout.Snapshot {
	nodes := make(map[string]layout.Node, len(ids))
	for _, id := range ids {
		nodes[id] = layout.Node{}
	}
	return layout.Snapshot{Nodes: nodes, SelectedID: selected}
}

func TestRelatedNodesWaitWhileSelectedAppearsFirst(t *testing.T) {
	c := newTestCanvas()
	first := snapshotOf("a", "a")
	c.markNewNodes(first)
	c.snapshot = first
	c.fadeStart = map[string]time.Time{}

	next := snapshotOf("a", "a", "b", "c")
	c.markNewNodes(next)

	if got := c.fadeAlpha("b"); got != 0 {
		t.Fatalf("related node alpha = %v, want 0 during delay", got)
	}
	if got := c.fadeAlpha("c"); got != 0 {
		t.Fatalf("related node alpha = %v, want 0 during delay", got)
	}
	if got := c.fadeAlpha("a"); got != 1 {
		t.Fatalf("existing node alpha = %v, want 1", got)
	}
}

func TestSelectionChangeHidesOthersUntilDelay(t *testing.T) {
c := newTestCanvas()
first := snapshotOf("a", "a", "b", "c", "d", "e")
c.markNewNodes(first)
c.snapshot = first
c.fadeStart = map[string]time.Time{}

c.markNewNodes(snapshotOf("b", "a", "b", "c", "d", "e"))

if got := c.fadeAlpha("b"); got != 1 {
t.Fatalf("selected alpha = %v, want 1", got)
}
for _, id := range []string{"a", "c", "d", "e"} {
if got := c.fadeAlpha(id); got != 0 {
t.Fatalf("%s alpha = %v, want 0 during delay", id, got)
}
}
}
