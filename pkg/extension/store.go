package extension

import (
	"context"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"os"
	"time"
)

func Store(ctx context.Context, name string) (*runtimepath.Root, *os.File, error) {
	r, e := runtimepath.OpenState()
	if e != nil {
		return nil, nil, e
	}
	c, e := r.Child(name)
	_ = r.Close()
	if e != nil {
		return nil, nil, e
	}
	for {
		lock, err := c.Lock("writer")
		if err == nil {
			return c, lock, nil
		}
		if !busy(err) {
			_ = c.Close()
			return nil, nil, err
		}
		select {
		case <-ctx.Done():
			_ = c.Close()
			return nil, nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}
