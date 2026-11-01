//go:build darwin && cgo

package metrics

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestDarwinProbeCounterUnits(t *testing.T) {
	// Keep enough resident memory to distinguish bytes from KiB independently
	// of small allocator/ps fluctuations. This is our own test process only.
	memory := make([]byte, 32<<20)
	for i := 0; i < len(memory); i += os.Getpagesize() {
		memory[i] = 1
	}
	cpu := func() time.Duration {
		var usage unix.Rusage
		if err := unix.Getrusage(unix.RUSAGE_SELF, &usage); err != nil {
			t.Fatal(err)
		}
		return time.Duration(usage.Utime.Nano() + usage.Stime.Nano())
	}
	// Accumulate a measurable CPU baseline so a clock-unit error cannot hide
	// behind the timestamp resolution of an almost-idle process.
	until := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(until) {
	}
	before := cpu()
	sample, err := probe(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	after := cpu()
	if sample.CPU < before-time.Millisecond || sample.CPU > after+time.Millisecond {
		t.Fatalf("libproc CPU %s outside getrusage interval [%s, %s]", sample.CPU, before, after)
	}
	if sample.Parent != os.Getppid() || sample.Identity == "" || sample.At.IsZero() {
		t.Fatal("missing process identity", sample)
	}
	output, err := exec.Command("/bin/ps", "-o", "rss=", "-p", strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		t.Fatal(err)
	}
	kib, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil || kib == 0 {
		t.Fatal("invalid ps RSS", string(output), err)
	}
	if sample.RSS < kib*1024/2 || sample.RSS > kib*1024*2 {
		t.Fatalf("libproc RSS %d bytes disagrees with ps RSS %d KiB", sample.RSS, kib)
	}
	runtime.KeepAlive(memory)
}

func TestDarwinProbeChildIdentityAndExit(t *testing.T) {
	child := exec.Command("/bin/sleep", "60")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if child.ProcessState == nil {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
	}()
	first, err := probe(child.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	second, err := probe(child.Process.Pid)
	if err != nil || first.Parent != os.Getpid() || first.Identity != second.Identity || first.Identity == "" {
		t.Fatal("unstable direct-child identity", first, second, err)
	}
	if err = child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	if _, err = probe(child.Process.Pid); err == nil {
		t.Fatal("sampled reaped child")
	}
	if _, err = probe(-1); err == nil {
		t.Fatal("sampled invalid PID")
	}
}
