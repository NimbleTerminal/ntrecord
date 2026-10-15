// Package doctor checks that this machine can record: the linked terminal
// engine, the renderer, a PTY, the terminfo entry, and the optional FFmpeg.
package doctor

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/creack/pty"
	"nimbleterminal.dev/ntrecord/internal/encode"
	"nimbleterminal.dev/ntrecord/internal/tape"
	"nimbleterminal.dev/ntrecord/internal/terminfo"
	"nimbleterminal.dev/ntrecord/render"
)

// Options configure a run. The zero value checks the defaults.
type Options struct {
	Version string // the ntrecord version to report
	Shell   string // the shell to check; default /bin/sh
}

type status int

const (
	ok status = iota
	warn
	fail
)

type result struct {
	name   string
	status status
	detail string
}

// Run prints one line per check and a summary. It returns false if any check
// failed; warnings, such as a missing optional FFmpeg, do not fail it.
func Run(w io.Writer, o Options) bool {
	if o.Version == "" {
		o.Version = "dev"
	}
	if o.Shell == "" {
		o.Shell = tape.DefaultConfig().Shell
	}
	results := []result{
		{"ntrecord", ok, fmt.Sprintf("%s %s/%s", o.Version, runtime.GOOS, runtime.GOARCH)},
		checkEngine(),
		checkRenderer(),
		checkPTY(o.Shell),
		checkTerminfo(),
		checkFFmpeg(),
	}
	var warns, fails int
	for _, r := range results {
		label := map[status]string{ok: "ok  ", warn: "warn", fail: "FAIL"}[r.status]
		fmt.Fprintf(w, "%s  %-9s %s\n", label, r.name, r.detail)
		switch r.status {
		case warn:
			warns++
		case fail:
			fails++
		}
	}
	switch {
	case fails > 0:
		fmt.Fprintf(w, "\n%d check(s) failed: ntrecord cannot record on this machine until they are fixed.\n", fails)
	case warns > 0:
		fmt.Fprintf(w, "\nReady to record, with %d warning(s) above.\n", warns)
	default:
		fmt.Fprintln(w, "\nAll checks passed.")
	}
	return fails == 0
}

func checkEngine() result {
	e, err := render.EngineInfo()
	switch {
	case err != nil:
		return result{"engine", fail, "libghostty-vt: " + err.Error()}
	case !e.KittyGraphics:
		return result{"engine", fail, "libghostty-vt " + e.Version + " was built without Kitty graphics"}
	}
	simd := "no SIMD"
	if e.SIMD {
		simd = "SIMD"
	}
	return result{"engine", ok, fmt.Sprintf("libghostty-vt %s (Kitty graphics, %s)", e.Version, simd)}
}

func checkRenderer() result {
	c := render.DefaultConfig()
	r, err := render.New(c)
	if err != nil {
		return result{"renderer", fail, err.Error()}
	}
	defer r.Close()
	if err = r.Write([]byte("ntrecord \x1b[31mdoctor\x1b[m\r\n")); err != nil {
		return result{"renderer", fail, err.Error()}
	}
	frame, err := r.Frame()
	if err != nil {
		return result{"renderer", fail, err.Error()}
	}
	b := frame.Bounds()
	return result{"renderer", ok, fmt.Sprintf("%s %gpx, %dx%d cells drew a %dx%d frame", c.FontFamily, c.FontSize, r.Cols, r.Rows, b.Dx(), b.Dy())}
}

// checkPTY runs the shell on a real PTY and reads its output back.
func checkPTY(shell string) result {
	path, err := exec.LookPath(shell)
	if err != nil {
		return result{"pty", fail, fmt.Sprintf("shell %s not found: %v", shell, err)}
	}
	cmd := exec.Command(path, "-c", "printf ntrecord-ok")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return result{"pty", fail, "cannot open a PTY: " + err.Error()}
	}
	defer ptmx.Close()
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		buf := make([]byte, 256)
		for !strings.Contains(b.String(), "ntrecord-ok") {
			n, err := ptmx.Read(buf)
			b.Write(buf[:n])
			if err != nil {
				break
			}
		}
		done <- b.String()
	}()
	select {
	case out := <-done:
		_ = cmd.Wait()
		if strings.Contains(out, "ntrecord-ok") {
			return result{"pty", ok, fmt.Sprintf("%s ran on a PTY", path)}
		}
		return result{"pty", fail, fmt.Sprintf("%s ran but its output did not arrive: %q", path, out)}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return result{"pty", fail, path + " produced no output on a PTY within 5s"}
	}
}

// checkTerminfo installs the embedded entry and asks ncurses to find it.
func checkTerminfo() result {
	dir, err := os.MkdirTemp("", "ntrecord-doctor-")
	if err != nil {
		return result{"terminfo", fail, err.Error()}
	}
	defer os.RemoveAll(dir)
	if err = terminfo.Install(dir); err != nil {
		return result{"terminfo", fail, err.Error()}
	}
	tput, err := exec.LookPath("tput")
	if err != nil {
		return result{"terminfo", warn, terminfo.Term + " entry installed; tput not found, so it was not verified"}
	}
	cmd := exec.Command(tput, "-T", terminfo.Term, "colors")
	cmd.Env = append(os.Environ(), "TERMINFO="+dir)
	out, err := cmd.Output()
	if got := strings.TrimSpace(string(out)); err != nil || got != "256" {
		return result{"terminfo", fail, fmt.Sprintf("tput could not read the %s entry (got %q, %v)", terminfo.Term, got, err)}
	}
	return result{"terminfo", ok, terminfo.Term + " entry readable by tput"}
}

// checkFFmpeg is optional: only MP4 and WebM output need it.
func checkFFmpeg() result {
	info, err := encode.FFmpegInfo()
	if err != nil {
		return result{"ffmpeg", warn, "not found: MP4 and WebM output are unavailable (GIF, text and PNG frames are unaffected)"}
	}
	var missing []string
	for _, c := range []struct{ format, codec string }{{"MP4", "libx264"}, {"WebM", "libvpx-vp9"}} {
		if !info.Encoders[c.codec] {
			missing = append(missing, c.format+" needs "+c.codec)
		}
	}
	if len(missing) > 0 {
		return result{"ffmpeg", warn, fmt.Sprintf("%s at %s lacks encoders: %s", info.Version, info.Path, strings.Join(missing, ", "))}
	}
	return result{"ffmpeg", ok, fmt.Sprintf("%s (libx264, libvpx-vp9)", info.Version)}
}
