//go:build darwin && cgo

package nativeprocess

import (
	"os"

	"golang.org/x/sys/unix"
)

// SignalGroup requires an unreaped direct child reserving the PGID. Darwin's
// killpg returns EPERM for a group containing only zombies. Accept that case
// only after kernel observations confirm the owned zombie leader and no live
// members. A real permission error or unavailable observation remains an error.
func SignalGroup(pid int, signal unix.Signal) error {
	err := unix.Kill(-pid, signal)
	if err != unix.EPERM {
		return err
	}
	exited, observeErr := Exited(pid)
	if observeErr != nil || !exited {
		return err
	}
	members, observeErr := unix.SysctlKinfoProcSlice("kern.proc.pgrp", pid)
	if observeErr != nil || len(members) == 0 {
		return err
	}
	ownedLeader := false
	for _, member := range members {
		// SZOMB is defined as 5 in Darwin <sys/proc.h>.
		if member.Proc.P_stat != 5 || int(member.Eproc.Pgid) != pid {
			return err
		}
		if int(member.Proc.P_pid) == pid && int(member.Eproc.Ppid) == os.Getpid() {
			ownedLeader = true
		}
	}
	if ownedLeader {
		return nil
	}
	return err
}
