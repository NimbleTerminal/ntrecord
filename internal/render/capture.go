package render

import (
	"image"
	"time"
)

// libghostty removes an image at the first chunk of a same-ID replacement.
// Its pinned API exposes neither that upload nor its completion as an effect.
// Allow a short gap for replacements without hiding a real deletion forever.
const imageReplacementGrace = 250 * time.Millisecond

// CaptureFrameAt samples a viewport for recording. If previously displayed
// virtual-image cells still reference an image that temporarily disappeared,
// it holds the last presented bitmap for at most imageReplacementGrace. This
// uses parsed cell IDs, never another parser for the PTY byte stream. FrameAt
// remains an immediate snapshot for screenshots and terminal-state inspection.
// Returned bitmaps are immutable, just like FrameAt's.
func (r *Renderer) CaptureFrameAt(now time.Time) (*image.RGBA, error) {
	current, err := r.FrameAt(now)
	if err != nil {
		return nil, err
	}
	missing := false
	for id := range r.frameMissingImages {
		if _, ok := r.presentedImages[id]; ok {
			missing = true
			break
		}
	}
	if missing && r.presented != nil {
		if r.imageGapUntil.IsZero() {
			r.imageGapUntil = now.Add(imageReplacementGrace)
		}
		if now.Before(r.imageGapUntil) {
			return r.presented, nil
		}
	}
	// Cleared/overwritten placeholders, completed replacements, and expired
	// gaps are presented normally. A timed-out gap cannot restart until an
	// actual image has been presented again.
	r.imageGapUntil = time.Time{}
	r.presented, r.presentedImages = current, r.frameImages
	return current, nil
}
