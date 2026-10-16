package ui

import (
	"time"

	"github.com/lxn/walk"
	"genealogymapper/internal/layout"
)

const (
	// fadeDuration is how long a newly appearing node takes to reach full opacity.
	fadeDuration = 2 * time.Second
	// fadeFrameInterval is the repaint cadence while any fade is running (~60 fps).
	fadeFrameInterval = 16 * time.Millisecond
	// relatedNodeDelay postpones the fade of neighbours so the selected node appears first.
	relatedNodeDelay = 500 * time.Millisecond
)

// markNewNodes records a fade start time for every node that should animate in.
//
// Nodes fade in when they were absent from the previous snapshot, or when the
// selection changed (in which case every node except the newly selected one
// re-fades). While a node is selected, related nodes start relatedNodeDelay
// later than the selected node. Start times may lie in the future; fadeAlpha
// treats those as fully transparent.
func (c *graphCanvas) markNewNodes(snapshot layout.Snapshot) {
	// An empty graph resets all animation state so the next load starts fresh.
	if len(snapshot.Nodes) == 0 {
		c.previousIDs = make(map[string]bool)
		c.fadeStart = make(map[string]time.Time)
		return
	}
	now := time.Now()
	relatedStart := now
	if snapshot.SelectedID != "" {
		relatedStart = now.Add(relatedNodeDelay)
	}
	// The very first snapshot is not a "selection change"; there is nothing to compare with.
	selectionChanged := len(c.previousIDs) > 0 && snapshot.SelectedID != c.snapshot.SelectedID
	nextIDs := make(map[string]bool, len(snapshot.Nodes))
	for id := range snapshot.Nodes {
		nextIDs[id] = true
		if selectionChanged {
			// The selected node stays solid; everything else fades back in.
			if id == snapshot.SelectedID {
				delete(c.fadeStart, id)
			} else {
				c.fadeStart[id] = relatedStart
			}
			continue
		}
		if !c.previousIDs[id] {
			// Do not restart a fade that is already in progress for this node.
			if _, fading := c.fadeStart[id]; !fading {
				if id == snapshot.SelectedID {
					c.fadeStart[id] = now
				} else {
					c.fadeStart[id] = relatedStart
				}
			}
		}
	}
	c.previousIDs = nextIDs
}

// startFadeLoop schedules repeated repaints until every pending fade has completed.
// It is a no-op when a loop is already running, so callers may invoke it freely.
func (c *graphCanvas) startFadeLoop() {
	if c.animating {
		return
	}
	c.animating = true
	c.scheduleFadeTick()
}

// scheduleFadeTick queues one animation frame. The timer fires on a background
// goroutine, so the actual work is marshalled onto the UI thread via Synchronize.
func (c *graphCanvas) scheduleFadeTick() {
	time.AfterFunc(fadeFrameInterval, func() {
		c.widget.Synchronize(func() {
			// Stop quietly if the window was closed while the timer was pending.
			if c.widget.IsDisposed() {
				c.animating = false
				return
			}
			now := time.Now()
			// Drop finished fades so fadeAlpha short-circuits to 1 for them.
			for id, start := range c.fadeStart {
				if now.Sub(start) >= fadeDuration {
					delete(c.fadeStart, id)
				}
			}
			_ = c.widget.Invalidate()
			if len(c.fadeStart) > 0 {
				c.scheduleFadeTick()
			} else {
				c.animating = false
			}
		})
	})
}

// fadeAlpha returns the fade-in progress for id, from 0 (just appeared) to 1 (fully visible).
// The curve is ease-out quadratic: fast at first, settling gently at full opacity.
func (c *graphCanvas) fadeAlpha(id string) float64 {
	start, fading := c.fadeStart[id]
	if !fading {
		return 1
	}
	elapsed := time.Since(start)
	// A future start time (delayed related node) means the node is still invisible.
	if elapsed <= 0 {
		return 0
	}
	if elapsed >= fadeDuration {
		return 1
	}
	t := float64(elapsed) / float64(fadeDuration)
	return 1 - (1-t)*(1-t)
}

// lerpColor linearly interpolates each RGB channel from one colour to another; t is in [0, 1].
// It is used to simulate transparency by blending towards the canvas background.
func lerpColor(from, to walk.Color, t float64) walk.Color {
	r := lerpByte(from.R(), to.R(), t)
	g := lerpByte(from.G(), to.G(), t)
	b := lerpByte(from.B(), to.B(), t)
	return walk.RGB(r, g, b)
}

// lerpByte interpolates a single colour channel.
func lerpByte(from, to byte, t float64) byte {
	return byte(float64(from) + (float64(to)-float64(from))*t)
}
