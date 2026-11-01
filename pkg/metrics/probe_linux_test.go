//go:build linux && cgo

package metrics

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestLinuxStatWithParenthesesAndClockUnits(t *testing.T) {
	f := make([]string, 22)
	for i := range f {
		f[i] = "0"
	}
	f[0] = "S"
	f[11] = "150"
	f[12] = "50"
	f[19] = "12345"
	f[21] = "3"
	value, e := parseLinuxStat("42 (worker ) name) "+strings.Join(f, " "), 100, 4096, time.Now())
	if e != nil || value.Identity != "12345" || value.CPU != 2*time.Second || value.RSS != 12288 {
		t.Fatal(value, e)
	}
	for _, state := range []string{"Z", "X"} {
		f[0] = state
		if _, e = parseLinuxStat(fmt.Sprintf("42 (x) %s", strings.Join(f, " ")), 100, 4096, time.Now()); e == nil {
			t.Fatal("sampled exited PID")
		}
	}
}
