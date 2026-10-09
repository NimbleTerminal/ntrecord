package record

import (
	"os"
	"syscall"
	"time"
)

// pollablePTY duplicates the master into a nonblocking, runtime-polled file.
// creack/pty returns a blocking descriptor (its setup ioctls call Fd), and on
// macOS its original os.NewFile is not registered with the runtime poller at
// all. Set O_NONBLOCK before NewFile so Close interrupts both reads and writes,
// even when a child or descendant holds the slave open without consuming input.
// The caller retains ownership of master and must close it before doing I/O.
func pollablePTY(master *os.File) (*os.File, error) {
	syscall.ForkLock.RLock()
	fd, err := syscall.Dup(int(master.Fd()))
	if err == nil {
		syscall.CloseOnExec(fd)
	}
	syscall.ForkLock.RUnlock()
	if err != nil {
		return nil, err
	}
	if err := syscall.SetNonblock(fd, true); err != nil {
		_ = syscall.Close(fd)
		return nil, err
	}
	f := os.NewFile(uintptr(fd), master.Name())
	// Fail during setup if the platform cannot poll this descriptor, instead of
	// risking an uninterruptible syscall during recording shutdown.
	if err := f.SetDeadline(time.Time{}); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}
