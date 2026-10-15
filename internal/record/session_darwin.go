package record

import "golang.org/x/sys/unix"

// processes lists the process IDs on this machine.
func processes() []int {
	procs, _ := unix.SysctlKinfoProcSlice("kern.proc.all")
	pids := make([]int, 0, len(procs))
	for _, p := range procs {
		pids = append(pids, int(p.Proc.P_pid))
	}
	return pids
}
