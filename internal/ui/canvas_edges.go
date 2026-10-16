package ui

import (
	"math"
	"sort"

	"github.com/lxn/walk"
	"genealogymapper/internal/layout"
)

const (
	hopRadius      = 7.0
	hopSegments    = 10
	junctionMinor  = 3
	junctionMajor  = 5
	horizontalSlop = 0.5
)

// edgeSegment is a straight screen-space line belonging to one edge.
// owners are the parent ids feeding it; segments sharing an owner are one source and never hop each other.
type edgeSegment struct {
	from, to layout.Point
	owners   []string
}

type junctionDot struct {
	at     layout.Point
	radius int
}

type edgeShape struct {
	alpha    float64
	segments []edgeSegment
	arrow    walk.Point
	arrowUp  bool
	dots     []junctionDot
}

// buildEdge computes the screen-space lines for a child and its parents.
// One parent gives a straight arrow; several give stubs to a shared bar and a connector to the child.
func (c *graphCanvas) buildEdge(_ string, parents []layout.Node, child layout.Node, alpha float64) edgeShape {
	end := c.viewport.Screen(layout.Point{X: child.Center.X, Y: child.Bounds.Y})
	shape := edgeShape{alpha: alpha, arrow: walk.Point{X: int(end.X), Y: int(end.Y)}}
	owners := make([]string, 0, len(parents))
	for _, parent := range parents {
		owners = append(owners, parent.Person.ID)
	}

	if len(parents) == 1 {
		start := c.viewport.Screen(layout.Point{X: parents[0].Center.X, Y: parents[0].Bounds.Y + parents[0].Bounds.Height})
		shape.segments = []edgeSegment{{from: start, to: end, owners: owners}}
		shape.arrowUp = end.Y < start.Y
		return shape
	}

	// Place the bar a quarter of a row gap beneath the lowest parent.
	lowest := math.Inf(-1)
	for _, parent := range parents {
		lowest = math.Max(lowest, parent.Bounds.Y+parent.Bounds.Height)
	}
	barY := c.viewport.Screen(layout.Point{X: 0, Y: lowest + layout.VerticalGap/4}).Y

	minX, maxX := end.X, end.X
	for _, parent := range parents {
		bottom := c.viewport.Screen(layout.Point{X: parent.Center.X, Y: parent.Bounds.Y + parent.Bounds.Height})
		shape.segments = append(shape.segments, edgeSegment{
			from:   bottom,
			to:     layout.Point{X: bottom.X, Y: barY},
			owners: []string{parent.Person.ID},
		})
		shape.dots = append(shape.dots, junctionDot{at: layout.Point{X: bottom.X, Y: barY}, radius: junctionMinor})
		minX = math.Min(minX, bottom.X)
		maxX = math.Max(maxX, bottom.X)
	}
	shape.segments = append(shape.segments,
		edgeSegment{from: layout.Point{X: minX, Y: barY}, to: layout.Point{X: maxX, Y: barY}, owners: owners},
		edgeSegment{from: layout.Point{X: end.X, Y: barY}, to: end, owners: owners},
	)
	shape.dots = append(shape.dots, junctionDot{at: layout.Point{X: end.X, Y: barY}, radius: junctionMajor})
	shape.arrowUp = end.Y < barY
	return shape
}

// drawEdgeShape draws shapes[index]; horizontal lines hop over unrelated edges they cross.
func (c *graphCanvas) drawEdgeShape(canvas *walk.Canvas, shapes []edgeShape, index int) {
	shape := shapes[index]
	pen, release := c.edgePen(shape.alpha)
	if pen == nil {
		return
	}
	defer release()

	for _, segment := range shape.segments {
		if math.Abs(segment.from.Y-segment.to.Y) < horizontalSlop {
			drawHoppingLine(canvas, pen, segment, shapes, index)
			continue
		}
		_ = canvas.DrawLinePixels(pen, toPoint(segment.from), toPoint(segment.to))
	}
	drawArrowHead(canvas, pen, shape.arrow, shape.arrowUp)

	if len(shape.dots) == 0 {
		return
	}
	brush, err := walk.NewSolidColorBrush(lerpColor(c.backgroundColor, c.edgeColor, shape.alpha))
	if err != nil {
		return
	}
	defer brush.Dispose()
	for _, dot := range shape.dots {
		_ = canvas.FillEllipsePixels(brush, walk.Rectangle{
			X:      int(dot.at.X) - dot.radius,
			Y:      int(dot.at.Y) - dot.radius,
			Width:  dot.radius * 2,
			Height: dot.radius * 2,
		})
	}
}

// drawHoppingLine draws a horizontal segment with a small arch over each unrelated edge it crosses.
func drawHoppingLine(canvas *walk.Canvas, pen *walk.CosmeticPen, segment edgeSegment, shapes []edgeShape, self int) {
	y := segment.from.Y
	left, right := segment.from.X, segment.to.X
	if left > right {
		left, right = right, left
	}

	var crossings []float64
	for index, other := range shapes {
		if index == self {
			continue
		}
		for _, candidate := range other.segments {
			if sharesOwner(segment.owners, candidate.owners) {
				continue
			}
			if x, ok := crossingX(y, candidate); ok && x-hopRadius > left && x+hopRadius < right {
				crossings = append(crossings, x)
			}
		}
	}
	sort.Float64s(crossings)

	cursor := left
	for _, x := range crossings {
		if x-hopRadius < cursor {
			continue
		}
		_ = canvas.DrawLinePixels(pen, walk.Point{X: int(cursor), Y: int(y)}, walk.Point{X: int(x - hopRadius), Y: int(y)})
		arch := make([]walk.Point, 0, hopSegments+1)
		for step := 0; step <= hopSegments; step++ {
			angle := math.Pi * (1 - float64(step)/hopSegments)
			arch = append(arch, walk.Point{
				X: int(x + hopRadius*math.Cos(angle)),
				Y: int(y - hopRadius*math.Sin(angle)),
			})
		}
		_ = canvas.DrawPolylinePixels(pen, arch)
		cursor = x + hopRadius
	}
	_ = canvas.DrawLinePixels(pen, walk.Point{X: int(cursor), Y: int(y)}, walk.Point{X: int(right), Y: int(y)})
}

// crossingX returns where a non-horizontal segment strictly crosses the horizontal line at y.
func crossingX(y float64, segment edgeSegment) (float64, bool) {
	dy := segment.to.Y - segment.from.Y
	if math.Abs(dy) < horizontalSlop {
		return 0, false
	}
	t := (y - segment.from.Y) / dy
	if t <= 0.02 || t >= 0.98 {
		return 0, false
	}
	return segment.from.X + t*(segment.to.X-segment.from.X), true
}

func sharesOwner(a, b []string) bool {
	for _, left := range a {
		for _, right := range b {
			if left == right {
				return true
			}
		}
	}
	return false
}

func toPoint(point layout.Point) walk.Point {
	return walk.Point{X: int(point.X), Y: int(point.Y)}
}
