//go:build (linux || darwin) && cgo

// Package nativeprocess observes child exit without releasing its PID before
// group teardown. The command server already requires CGO for libghostty.
package nativeprocess

/*
#include <sys/wait.h>
#include <errno.h>
static int txm_wait_exited(int pid) {
 siginfo_t info;
 int result;
 do { result = waitid(P_PID, (id_t)pid, &info, WEXITED | WNOWAIT); }
 while (result < 0 && errno == EINTR);
 return result < 0 ? errno : 0;
}
static int txm_has_exited(int pid, int *exited) {
 siginfo_t info = {0};
 int result;
 do { result = waitid(P_PID, (id_t)pid, &info, WEXITED | WNOWAIT | WNOHANG); }
 while (result < 0 && errno == EINTR);
 *exited = info.si_pid != 0;
 return result < 0 ? errno : 0;
}
*/
import "C"
import "syscall"

// WaitExited leaves the child waitable, reserving its PID/process group ID until
// the owner has sent its last group signal and calls exec.Cmd.Wait to reap it.
func WaitExited(pid int) error {
	if e := C.txm_wait_exited(C.int(pid)); e != 0 {
		return syscall.Errno(e)
	}
	return nil
}

// Exited obtains a current kernel observation without reaping the child.
func Exited(pid int) (bool, error) {
	var exited C.int
	if e := C.txm_has_exited(C.int(pid), &exited); e != 0 {
		return false, syscall.Errno(e)
	}
	return exited != 0, nil
}
