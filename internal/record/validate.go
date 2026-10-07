package record

import (
	"errors"
	"fmt"
	"os"

	"github.com/NimbleMarkets/sigmamax/internal/encode"
	"github.com/NimbleMarkets/sigmamax/internal/render"
	"github.com/NimbleMarkets/sigmamax/internal/tape"
)

// Summary is what a tape would record, found without starting a PTY.
type Summary struct {
	Commands      int
	Outputs       []string
	Cols, Rows    int
	Width, Height int // output pixels, decorations included
}

// Validate checks everything about t that can be checked without running it:
// the renderer builds (fonts, theme, size limits), each Require is on the
// recording's PATH, and each Output can be written (video needs FFmpeg with its
// encoder). It reports all the problems it finds, and writes nothing.
func Validate(t *tape.Tape) (Summary, error) {
	c := t.Config
	s := Summary{Commands: len(t.Actions), Outputs: c.Destinations()}
	var errs []error
	errs = append(errs, validateCast(c, t))
	if r, err := render.New(c); err != nil {
		errs = append(errs, err)
	} else {
		s.Cols, s.Rows = r.Cols, r.Rows
		s.Width, s.Height = r.OutputSize()
		r.Close()
	}
	env := recordingEnv(c, "")
	for _, program := range c.Require {
		if _, err := lookPath(program, env); err != nil {
			errs = append(errs, fmt.Errorf("Require %s: %w", program, err))
		}
	}
	for _, path := range s.Outputs {
		if _, err := encode.Preflight(path); err != nil {
			errs = append(errs, err)
		}
	}
	return s, errors.Join(errs...)
}

// recordingEnv is the environment the recorded program runs in: the process
// environment as environment() declares it, then the tape's Env commands.
func recordingEnv(c tape.Config, terminfoDir string) []string {
	env := environment(os.Environ(), c.ProgramVersion(), terminfoDir)
	for key, value := range c.Env {
		env = replaceEnv(env, key, value)
	}
	return env
}
