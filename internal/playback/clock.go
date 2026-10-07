package playback

import (
	"fmt"
	"math"
	"time"
)

// Clock advances in recorded time and preserves position across pause/speed
// changes. Callers supply monotonic wall time, which also makes it testable.
type Clock struct {
	Duration time.Duration
	Position time.Duration
	Speed    float64
	Paused   bool
	anchor   time.Time
}

func NewClock(duration time.Duration, speed float64, now time.Time) (Clock, error) {
	if !validSpeed(speed) {
		return Clock{}, fmt.Errorf("speed must be between 0.1 and 16")
	}
	return Clock{Duration: duration, Speed: speed, Paused: true, anchor: now}, nil
}

func validSpeed(speed float64) bool { return !math.IsNaN(speed) && speed >= 0.1 && speed <= 16 }

func (c *Clock) Update(now time.Time) {
	if !c.Paused {
		elapsed := max(now.Sub(c.anchor), 0)
		delta := float64(elapsed) * c.Speed
		if delta >= float64(c.Duration-c.Position) {
			c.Position = c.Duration
			c.Paused = true
		} else {
			c.Position += time.Duration(delta)
		}
	}
	c.anchor = now
}

func (c *Clock) Play(now time.Time) {
	c.Update(now)
	if c.Position == c.Duration {
		c.Position = 0
	}
	c.Paused = false
}
func (c *Clock) Pause(now time.Time) { c.Update(now); c.Paused = true }
func (c *Clock) Seek(at time.Duration, now time.Time) {
	c.Update(now)
	c.Position = min(max(at, 0), c.Duration)
	if c.Position == c.Duration {
		c.Paused = true
	}
}
func (c *Clock) SetSpeed(speed float64, now time.Time) error {
	if !validSpeed(speed) {
		return fmt.Errorf("speed must be between 0.1 and 16")
	}
	c.Update(now)
	c.Speed = speed
	return nil
}
