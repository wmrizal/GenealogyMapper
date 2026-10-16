package ui

import (
	"time"

	"github.com/lxn/walk"
	"genealogymapper/internal/layout"
)

// graphCanvas is a custom walk widget that draws the family graph and handles pan, zoom and selection.
//
// The implementation is split across files:
//   - canvas.go          construction, state and snapshot updates
//   - canvas_viewport.go fit-to-window, hit testing and mouse input
//   - canvas_paint.go    drawing of edges and nodes
//   - canvas_fade.go     fade-in animation and colour blending
//
// All methods must be called on the UI thread.
type graphCanvas struct {
	widget   *walk.CustomWidget
	snapshot layout.Snapshot // latest layout to render
	viewport layout.Viewport // world-to-screen transform (scale + offset)
	onSelect func(string)    // invoked with a person id when a node is clicked

	// Pan state, active while the middle or right button is held.
	dragging bool
	lastPos  layout.Point

	// Shared GDI resources, created once and reused every frame.
	font     *walk.Font
	regular  *walk.SolidColorBrush
	selected *walk.SolidColorBrush
	canvas   *walk.SolidColorBrush // background fill
	outline  *walk.CosmeticPen
	edge     *walk.CosmeticPen

	// Raw colours kept alongside the resources so fades can blend between them.
	regularColor    walk.Color
	selectedColor   walk.Color
	backgroundColor walk.Color
	outlineColor    walk.Color
	edgeColor       walk.Color
	textColor       walk.Color

	// Fade-in animation state.
	previousIDs map[string]bool      // node ids present in the previous snapshot
	fadeStart   map[string]time.Time // when each currently fading node began to appear
	animating   bool                 // true while a fade tick is scheduled
}

// newGraphCanvas creates the canvas widget inside parent and wires up its event handlers.
// onSelect may be nil.
func newGraphCanvas(parent walk.Container, onSelect func(string)) (*graphCanvas, error) {
	font, err := walk.NewFont("Segoe UI", 10, 0)
	if err != nil {
		return nil, err
	}
	regular, err := walk.NewSolidColorBrush(walk.RGB(232, 241, 237))
	if err != nil {
		return nil, err
	}
	selected, err := walk.NewSolidColorBrush(walk.RGB(246, 220, 162))
	if err != nil {
		return nil, err
	}
	background, err := walk.NewSolidColorBrush(walk.RGB(248, 247, 242))
	if err != nil {
		return nil, err
	}
	outline, err := walk.NewCosmeticPen(walk.PenSolid, walk.RGB(35, 57, 62))
	if err != nil {
		return nil, err
	}
	edge, err := walk.NewCosmeticPen(walk.PenSolid, walk.RGB(112, 126, 128))
	if err != nil {
		return nil, err
	}

	canvas := &graphCanvas{
		viewport:        layout.DefaultViewport(),
		onSelect:        onSelect,
		font:            font,
		regular:         regular,
		selected:        selected,
		canvas:          background,
		outline:         outline,
		edge:            edge,
		regularColor:    walk.RGB(232, 241, 237),
		selectedColor:   walk.RGB(246, 220, 162),
		backgroundColor: walk.RGB(248, 247, 242),
		outlineColor:    walk.RGB(35, 57, 62),
		edgeColor:       walk.RGB(112, 126, 128),
		textColor:       walk.RGB(35, 57, 62),
		previousIDs:     make(map[string]bool),
		fadeStart:       make(map[string]time.Time),
	}
	widget, err := walk.NewCustomWidgetPixels(parent, 0, canvas.paint)
	if err != nil {
		return nil, err
	}
	canvas.widget = widget
	// PaintBuffered draws off-screen; the deprecated SetDoubleBuffering(true) is only single-buffered.
	widget.SetPaintMode(walk.PaintBuffered)
	widget.MouseDown().Attach(canvas.mouseDown)
	widget.MouseMove().Attach(canvas.mouseMove)
	widget.MouseUp().Attach(canvas.mouseUp)
	widget.MouseWheel().Attach(canvas.mouseWheel)
	// Re-fit on resize so the whole graph stays visible.
	widget.SizeChanged().Attach(func() {
		canvas.fit()
		_ = widget.Invalidate()
	})
	return canvas, nil
}

// setSnapshot replaces the rendered layout, refits the view and starts fade-ins for new nodes.
func (c *graphCanvas) setSnapshot(snapshot layout.Snapshot) {
	// markNewNodes must run before c.snapshot is replaced: it compares against the old selection.
	c.markNewNodes(snapshot)
	c.snapshot = snapshot
	c.fit()
	_ = c.widget.Invalidate()
	if len(c.fadeStart) > 0 {
		c.startFadeLoop()
	}
}
