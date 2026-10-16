package ui

import (
	"math"

	"github.com/lxn/walk"
	"genealogymapper/internal/layout"
)

// fitPadding is the screen-space margin, in pixels, kept around the graph when fitting.
const fitPadding = 64.0

// fit scales and centres the viewport so every node is visible with padding.
// The scale is clamped to [layout.MinZoom, layout.MaxZoom]; an empty graph resets to the default viewport.
func (c *graphCanvas) fit() {
	if len(c.snapshot.Nodes) == 0 {
		c.viewport = layout.DefaultViewport()
		return
	}

	// Compute the bounding box of all nodes in world coordinates.
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, node := range c.snapshot.Nodes {
		minX = math.Min(minX, node.Bounds.X)
		minY = math.Min(minY, node.Bounds.Y)
		maxX = math.Max(maxX, node.Bounds.X+node.Bounds.Width)
		maxY = math.Max(maxY, node.Bounds.Y+node.Bounds.Height)
	}
	bounds := layout.Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}

	// Pick the largest scale at which the box fits both axes (never below 1px of space).
	client := c.widget.ClientBoundsPixels()
	availableWidth := math.Max(float64(client.Width)-fitPadding*2, 1)
	availableHeight := math.Max(float64(client.Height)-fitPadding*2, 1)
	scale := math.Min(availableWidth/bounds.Width, availableHeight/bounds.Height)
	if scale < layout.MinZoom {
		scale = layout.MinZoom
	}
	if scale > layout.MaxZoom {
		scale = layout.MaxZoom
	}

	// Offset the viewport so the graph centre lands on the widget centre.
	worldCenter := layout.Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
	screenCenter := layout.Point{X: float64(client.Width) / 2, Y: float64(client.Height) / 2}
	c.viewport = layout.Viewport{
		Scale: scale,
		Offset: layout.Point{
			X: screenCenter.X - worldCenter.X*scale,
			Y: screenCenter.Y - worldCenter.Y*scale,
		},
	}
}

// hitTest returns the id of the node under a screen position, or "" if there is none.
func (c *graphCanvas) hitTest(screen layout.Point) string {
	world := c.viewport.World(screen)
	for id, node := range c.snapshot.Nodes {
		if node.Bounds.Contains(world) {
			return id
		}
	}
	return ""
}

// mouseDown selects a node on left click, or begins panning on middle/right click.
func (c *graphCanvas) mouseDown(x, y int, button walk.MouseButton) {
	position := layout.Point{X: float64(x), Y: float64(y)}
	if button == walk.LeftButton {
		if id := c.hitTest(position); id != "" && c.onSelect != nil {
			c.onSelect(id)
			return
		}
	}
	if button == walk.MiddleButton || button == walk.RightButton {
		c.dragging = true
		c.lastPos = position
	}
}

// mouseMove pans the viewport by the pointer delta while a drag is active.
func (c *graphCanvas) mouseMove(x, y int, _ walk.MouseButton) {
	if !c.dragging {
		return
	}
	position := layout.Point{X: float64(x), Y: float64(y)}
	c.viewport.Offset.X += position.X - c.lastPos.X
	c.viewport.Offset.Y += position.Y - c.lastPos.Y
	c.lastPos = position
	_ = c.widget.Invalidate()
}

// mouseUp ends any active pan.
func (c *graphCanvas) mouseUp(_, _ int, _ walk.MouseButton) {
	c.dragging = false
}

// mouseWheel zooms in or out by 10% per notch, keeping the point under the cursor fixed.
func (c *graphCanvas) mouseWheel(x, y int, button walk.MouseButton) {
	// walk packs the wheel delta into the button argument.
	delta := walk.MouseWheelEventDelta(button)
	if delta == 0 {
		return
	}
	factor := 1.1
	if delta < 0 {
		factor = 1 / factor
	}
	c.viewport = c.viewport.ZoomAt(layout.Point{X: float64(x), Y: float64(y)}, factor)
	_ = c.widget.Invalidate()
}
