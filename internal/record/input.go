package record

import "time"

// liveInterrupt tracks consecutive Ctrl-C presses across input reads. Any
// other input disarms the shortcut, so an earlier interrupted command cannot
// cause a later Ctrl-C to stop the recording.
type liveInterrupt struct{ last time.Time }

func (s *liveInterrupt) stop(b []byte, now time.Time) bool {
	for _, c := range b {
		if c != 3 {
			s.last = time.Time{}
			continue
		}
		if !s.last.IsZero() && now.Sub(s.last) <= time.Second {
			return true
		}
		s.last = now
	}
	return false
}
