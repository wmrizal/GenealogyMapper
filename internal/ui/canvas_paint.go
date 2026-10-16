package ui

import (
	"math"
	"sort"

	"github.com/lxn/walk"
	"genealogymapper/internal/layout"
)

// paint renders one frame: background, parent-child edges, then nodes on top.
// Edges are drawn first so node boxes cover the ends of their lines.
func (c *graphCanvas) paint(canvas *walk.Canvas, _ walk.Rectangle) error {
	bounds := canvas.BoundsPixels()
	if err := canvas.FillRectanglePixels(c.canvas, bounds); err != nil {
		return err
	}

	// Group parents by child so a child with several parents gets one combined edge.
	parentsByChild := make(map[string][]layout.Node)
	for _, relation := range c.snapshot.ParentChild {
		if parent, ok := c.snapshot.Nodes[relation.ParentID]; ok {
			parentsByChild[relation.ChildID] = append(parentsByChild[relation.ChildID], parent)
		}
	}
	// Map order is random; sort so overlapping shapes stack identically every frame.
	childIDs := make([]string, 0, len(parentsByChild))
	for childID := range parentsByChild {
		childIDs = append(childIDs, childID)
	}
	sort.Strings(childIDs)
	shapes := make([]edgeShape, 0, len(childIDs))
	for _, childID := range childIDs {
		parents := parentsByChild[childID]
		child, childExists := c.snapshot.Nodes[childID]
		if !childExists {
			continue
		}
		// An edge is only as visible as the faintest node it connects.
		alpha := c.fadeAlpha(childID)
		for _, parent := range parents {
			alpha = math.Min(alpha, c.fadeAlpha(parent.Person.ID))
		}
		shapes = append(shapes, c.buildEdge(childID, parents, child, alpha))
	}
	for index := range shapes {
		c.drawEdgeShape(canvas, shapes, index)
	}
	nodeIDs := make([]string, 0, len(c.snapshot.Nodes))
	for id := range c.snapshot.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)
	for _, id := range nodeIDs {
		c.drawNode(canvas, id, c.snapshot.Nodes[id])
	}
	if len(c.snapshot.Nodes) == 0 {
		return canvas.DrawTextPixels("Create a person to begin your family graph", c.font, walk.RGB(75, 88, 90), bounds, walk.TextCenter|walk.TextVCenter|walk.TextSingleLine)
	}
	return nil
}

// edgePen returns a pen for the given fade alpha plus a release func the caller must invoke.
// At full opacity the shared pen is reused; otherwise a temporary blended pen is created.
// A nil pen means creation failed and the edge should be skipped.
func (c *graphCanvas) edgePen(alpha float64) (*walk.CosmeticPen, func()) {
	if alpha >= 1 {
		return c.edge, func() {}
	}
	blended, err := walk.NewCosmeticPen(walk.PenSolid, lerpColor(c.backgroundColor, c.edgeColor, alpha))
	if err != nil {
		return nil, func() {}
	}
	return blended, blended.Dispose
}

// drawArrowHead draws two short strokes forming a "V" at tip.
// upward selects whether the arrow points up (true) or down (false) on screen.
func drawArrowHead(canvas *walk.Canvas, pen *walk.CosmeticPen, tip walk.Point, upward bool) {
	direction := 1
	if upward {
		direction = -1
	}
	base := walk.Point{X: tip.X, Y: tip.Y - direction*10}
	_ = canvas.DrawLinePixels(pen, tip, walk.Point{X: base.X - 6, Y: base.Y})
	_ = canvas.DrawLinePixels(pen, tip, walk.Point{X: base.X + 6, Y: base.Y})
}

// drawNode paints a rounded box with the person's name, highlighting the selected node.
// While fading in, fill, outline and text colours are blended towards the background.
func (c *graphCanvas) drawNode(canvas *walk.Canvas, id string, node layout.Node) {
	// Convert the node's world-space bounds to screen pixels.
	worldBounds := node.Bounds
	screenTopLeft := c.viewport.Screen(layout.Point{X: worldBounds.X, Y: worldBounds.Y})
	screenBounds := walk.Rectangle{
		X:      int(screenTopLeft.X),
		Y:      int(screenTopLeft.Y),
		Width:  int(worldBounds.Width * c.viewport.Scale),
		Height: int(worldBounds.Height * c.viewport.Scale),
	}

	fillColor := c.regularColor
	brush := c.regular
	if id == c.snapshot.SelectedID {
		fillColor = c.selectedColor
		brush = c.selected
	}

	// Only allocate temporary blended resources while the node is actually fading.
	alpha := c.fadeAlpha(id)
	outline := c.outline
	textColor := c.textColor
	if alpha < 1 {
		blendedFill, err := walk.NewSolidColorBrush(lerpColor(c.backgroundColor, fillColor, alpha))
		if err == nil {
			defer blendedFill.Dispose()
			brush = blendedFill
		}
		blendedOutline, err := walk.NewCosmeticPen(walk.PenSolid, lerpColor(c.backgroundColor, c.outlineColor, alpha))
		if err == nil {
			defer blendedOutline.Dispose()
			outline = blendedOutline
		}
		textColor = lerpColor(c.backgroundColor, c.textColor, alpha)
	}

	_ = canvas.FillRoundedRectanglePixels(brush, screenBounds, walk.Size{Width: 12, Height: 12})
	_ = canvas.DrawRoundedRectanglePixels(outline, screenBounds, walk.Size{Width: 12, Height: 12})
	_ = canvas.DrawTextPixels(node.Person.Name, c.font, textColor, screenBounds, walk.TextCenter|walk.TextVCenter|walk.TextSingleLine)
}
