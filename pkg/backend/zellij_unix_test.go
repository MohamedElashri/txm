//go:build linux || darwin

package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestZellijBackgroundReadinessIsExactBoundedAndNeverRelaunched(t *testing.T) {
	for _, delayed := range []bool{true, false} {
		t.Run(fmt.Sprint(delayed), func(t *testing.T) {
			dir := t.TempDir()
			log, launched, counter := filepath.Join(dir, "calls"), filepath.Join(dir, "launched"), filepath.Join(dir, "counter")
			body := fmt.Sprintf(`printf '%%s\n' "$1" >> '%s'
case "$1" in
attach) printf ready > '%s'; exit 0;;
list-sessions)
 printf 'foobar\n'
 if [ -e '%s' ]; then
  count=0; [ ! -e '%s' ] || read -r count < '%s'
  count=$((count+1)); printf '%%s\n' "$count" > '%s'
  if [ "$count" -ge 3 ] && [ '%t' = true ]; then printf 'foo\n'; fi
 fi;;
esac
`, log, launched, launched, counter, counter, counter, delayed)
			executable(t, dir, "zellij", body)
			t.Setenv("PATH", dir)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			err := NewZellijBackend(ctx).CreateSession("foo")
			if delayed && err != nil || !delayed && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("incorrect readiness outcome", err)
			}
			data, err := os.ReadFile(log)
			minimumLists := 2 // Preflight plus readiness; deadline need not allow a fixed poll count.
			if delayed {
				minimumLists = 4 // Prefix-only observations cannot satisfy readiness.
			}
			if err != nil || strings.Count(string(data), "attach\n") != 1 || strings.Count(string(data), "list-sessions\n") < minimumLists {
				t.Fatal("creation was retried or exact observation skipped", string(data), err)
			}
		})
	}
}
