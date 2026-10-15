package record

import (
	"slices"

	"golang.org/x/sys/unix"
)

// sessionMembers lists the processes in session sid. Jobs a shell put in the
// background have process groups of their own, and not every shell hangs them
// up as it exits (dash does not), but they stay in its session, which keeps
// its ID after the shell is gone.
func sessionMembers(sid int) []int {
	var members []int
	for _, pid := range processes() {
		if got, err := unix.Getsid(pid); err == nil && got == sid && pid != unix.Getpid() {
			members = append(members, pid)
		}
	}
	return slices.Sorted(slices.Values(members))
}

// endSession signals every process in session sid but leader.
func endSession(sid int, sig unix.Signal) {
	for _, pid := range sessionMembers(sid) {
		if pid != sid {
			_ = unix.Kill(pid, sig)
		}
	}
}
