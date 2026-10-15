package record

import (
	"os"
	"strconv"
)

// processes lists the process IDs on this machine.
func processes() []int {
	entries, _ := os.ReadDir("/proc")
	var pids []int
	for _, e := range entries {
		if pid, err := strconv.Atoi(e.Name()); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}
