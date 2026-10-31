package core

import (
	"bytes"
	"encoding/base64"
	"strings"
	"time"
	"unicode/utf8"
)

// Plain strips ECMA-48 controls, including OSC/DCS/APC/PM strings and their
// unterminated tails. C1 controls are recognized as bytes or Unicode controls.
func Plain(data []byte) string {
	var out strings.Builder
	state := byte(0)
	for len(data) > 0 {
		r, n := utf8.DecodeRune(data)
		if r == utf8.RuneError && n == 1 {
			r = rune(data[0])
			if r < 0x80 || r > 0x9f {
				data = data[1:]
				continue
			}
		}
		data = data[n:]
		switch state {
		case 1: // ESC
			switch r {
			case '[':
				state = 2
			case ']', 'P', 'X', '^', '_':
				state = 3
			default:
				if r >= 0x20 && r <= 0x2f {
					state = 5
				} else {
					state = 0
				}
			}
		case 2:
			if r >= 0x40 && r <= 0x7e {
				state = 0
			}
		case 3:
			switch r {
			case 7, 0x9c:
				state = 0
			case 27:
				state = 4
			}
		case 4:
			if r == '\\' || r == 7 || r == 0x9c {
				state = 0
			} else if r != 27 {
				state = 3
			}
		case 5:
			if r >= 0x30 && r <= 0x7e {
				state = 0
			}
		default:
			switch r {
			case 27:
				state = 1
			case 0x9b:
				state = 2
			case 0x90, 0x98, 0x9d, 0x9e, 0x9f:
				state = 3
			case '\n', '\t':
				out.WriteRune(r)
			default:
				if r >= 0x20 && r != 0x7f && (r < 0x80 || r > 0x9f) {
					out.WriteRune(r)
				}
			}
		}
	}
	return out.String()
}
func LineCount(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		n++
	}
	return n
}
func MakeSnapshot(backend, name string, data []byte, o CaptureOptions, truncated bool) Snapshot {
	if !o.Raw {
		data = []byte(Plain(data))
	}
	// The provider bounds collection. Apply limits again after sanitization.
	if LineCount(data) > o.Lines {
		end := len(data)
		if end > 0 && data[end-1] == '\n' {
			end--
		}
		start := end
		for i := 0; i < o.Lines && start > 0; i++ {
			start = bytes.LastIndexByte(data[:start], '\n')
			if start < 0 {
				start = -1
				break
			}
		}
		data = data[start+1:]
		truncated = true
	}
	if len(data) > o.Bytes {
		data = data[:o.Bytes]
		truncated = true
		if !o.Raw {
			for len(data) > 0 && !utf8.Valid(data) {
				data = data[:len(data)-1]
			}
		}
	}
	s := Snapshot{Locator: SessionLocator{Backend: backend, Name: name}, Type: "screen_snapshot", Format: "plain", Content: string(data), Lines: LineCount(data), Bytes: len(data), Truncated: truncated, ObservedAt: time.Now().UTC()}
	if o.History {
		s.Type = "history"
	}
	if o.Raw {
		s.Format = "raw"
		s.Encoding = String("base64")
		s.Content = base64.StdEncoding.EncodeToString(data)
	}
	return s
}
