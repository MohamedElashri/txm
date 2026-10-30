package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

var normal = tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
var muted = normal.Foreground(tcell.ColorGray)
var accent = normal.Foreground(tcell.ColorLightCyan)
var warning = normal.Foreground(tcell.ColorYellow)
var selectedStyle = normal.Background(tcell.ColorDarkBlue)

// Bound every string and remove terminal controls, including untrusted names.
func safe(value string) string {
	r := []rune(core.Plain([]byte(value)))
	if len(r) > 2048 {
		r = r[:2048]
	}
	return strings.ReplaceAll(strings.ReplaceAll(string(r), "\n", " "), "\t", " ")
}
func put(s tcell.Screen, x, y, width int, value string, style tcell.Style) {
	w, h := s.Size()
	if x < 0 || y < 0 || y >= h || x >= w {
		return
	}
	width = min(width, w-x)
	if width <= 0 {
		return
	}
	text := uniseg.NewGraphemes(safe(value))
	used := 0
	for text.Next() {
		size := text.Width()
		if size < 1 {
			continue
		}
		if used+size > width {
			break
		}
		runes := text.Runes()
		s.SetContent(x+used, y, runes[0], runes[1:], style)
		used += size
	}
}
func Render(s tcell.Screen, m *Model, preview string, now time.Time) {
	s.Clear()
	w, h := s.Size()
	w = min(w, 512)
	h = min(h, 200)
	if w < 30 || h < 10 {
		put(s, 0, 0, w, "txm dashboard: enlarge terminal", warning)
		put(s, 0, 1, w, "q quit | Ctrl-C cancel", normal)
		s.Show()
		return
	}
	rows := m.Rows(now)
	choice := m.Selection(rows)
	filter := "all"
	if m.BackendFilter > 0 {
		filter = m.Options.Backends[m.BackendFilter-1]
	}
	mode := "agents off (--agents)"
	if m.Options.Agents {
		mode = "agents subscribed"
		if m.AgentError != nil || now.Sub(m.AgentUpdated) > AgentFreshness {
			mode = "agents stale / unavailable"
		}
	}
	put(s, 0, 0, w, "txm dashboard | local host "+m.Options.HostID+" | "+mode, accent)
	put(s, 0, 1, w, fmt.Sprintf("Search /%s | filter: %s | backend: %s | %d rows", m.Query, Filters[m.Filter], filter, len(rows)), normal)
	status := []string{}
	for _, name := range m.Options.Backends {
		st, ok := m.Backends[name]
		switch {
		case !ok:
			status = append(status, name+":loading")
		case st.Err != nil:
			status = append(status, name+":"+core.Normalize(st.Err).Code)
		case st.Truncated:
			status = append(status, name+":TRUNCATED")
		default:
			status = append(status, fmt.Sprintf("%s:%d", name, len(st.Sessions)))
		}
	}
	total := 0
	for _, b := range m.Backends {
		total += len(b.Sessions)
	}
	if total > MaxSessions {
		status = append(status, "session display truncated at 2048")
	}
	put(s, 0, 2, w, strings.Join(status, " | "), warning)
	listHeight := max(1, (h-10)/2)
	start := 0
	if m.Index >= listHeight {
		start = m.Index - listHeight + 1
	}
	cols := []struct {
		x, width int
		title    string
	}{{0, 8, "TYPE"}, {8, 9, "BACKEND"}, {17, max(10, w-72), "NAME"}, {max(27, w-55), 14, "PROCESS"}, {max(41, w-41), 14, "ATTENTION"}, {max(55, w-27), 15, "CLIENTS"}, {max(66, w-12), 12, "AGE"}}
	if w < 85 {
		cols = []struct {
			x, width int
			title    string
		}{{0, 8, "TYPE"}, {8, 9, "BACKEND"}, {17, max(1, w-34), "NAME"}, {w - 17, 17, "STATE"}}
	}
	for _, c := range cols {
		put(s, c.x, 3, c.width, c.title, muted)
	}
	for i := start; i < min(start+listHeight, len(rows)); i++ {
		r := rows[i]
		style := normal
		if i == m.Index {
			style = selectedStyle
		}
		if r.Stale {
			style = style.Foreground(tcell.ColorYellow)
		}
		for x := 0; x < w; x++ {
			s.SetContent(x, 4+i-start, ' ', nil, style)
		}
		kind, clients := "session", "unknown"
		var observed time.Time
		if r.Session != nil {
			clients = r.Session.ClientState
			observed = r.Session.ObservedAt
			if r.Session.AttachedClients != nil {
				clients = fmt.Sprintf("%s:%d", clients, *r.Session.AttachedClients)
			}
		} else {
			kind = "agent"
			observed = r.Agent.ObservedAt
		}
		state := r.Lifecycle()
		if r.Stale {
			state += " stale"
		}
		values := []string{kind, r.Backend(), r.Name(), state, r.Attention(), clients, age(now, observed)}
		if w < 85 {
			values = []string{kind, r.Backend(), r.Name(), state}
		}
		for j, c := range cols {
			put(s, c.x, 4+i-start, c.width, values[j], style)
		}
	}
	if len(rows) == 0 {
		put(s, 0, 4, w, "No matching observations. r refresh | / search | Tab filter", muted)
	}
	detailY := 4 + listHeight
	put(s, 0, detailY, w, strings.Repeat("-", min(w, 512)), muted)
	detail := []string{}
	if choice != nil {
		if v := choice.Session; v != nil {
			id := "unknown"
			if v.BackendID != nil {
				id = *v.BackendID
			}
			detail = append(detail, "Session "+v.Locator.Backend+"/"+v.Locator.Name+" | backend ID "+id+" (backend scoped)")
			detail = append(detail, "Source: "+v.Source+" | observed "+v.ObservedAt.UTC().Format(time.RFC3339)+" | management/attention: unassociated")
			caps := []string{}
			for _, k := range []string{"attach", "read_only_attach", "output_snapshot", "output_history", "managed_launch"} {
				level := "unknown"
				if c, ok := v.Capabilities[k]; ok {
					level = c.Level
				}
				caps = append(caps, k+"="+level)
			}
			detail = append(detail, strings.Join(caps, " | "))
		} else {
			v := choice.Agent
			mode := "registered (unsupervised)"
			if v.Supervised {
				mode = "managed (supervised)"
			}
			detail = append(detail, mode+" | ID "+v.ID+" | "+v.Locator.Backend+"/"+v.Locator.Name)
			detail = append(detail, "Process: "+choice.Lifecycle()+" | source: "+v.Source+" | observed "+v.ObservedAt.UTC().Format(time.RFC3339))
			attentionTime := "unknown"
			if v.AttentionObservedAt != nil {
				attentionTime = v.AttentionObservedAt.UTC().Format(time.RFC3339)
			}
			detail = append(detail, "Attention: "+choice.Attention()+" | source: "+v.AttentionSource+" | observed "+attentionTime)
			extra := "Receipt: none | turn complete: unknown"
			if v.Receipt != nil {
				extra = fmt.Sprintf("Receipt: %s", v.Receipt.Outcome)
				if v.Receipt.ExitCode != nil {
					extra += fmt.Sprintf(" code=%d", *v.Receipt.ExitCode)
				}
			}
			if v.TurnCompleteAt != nil {
				extra += " | turn complete: " + v.TurnCompleteAt.UTC().Format(time.RFC3339)
			}
			detail = append(detail, extra+" | capabilities/client state unknown (select a session row)")
		}
	}
	for i, line := range detail {
		put(s, 0, detailY+1+i, w, line, normal)
	}
	previewY := detailY + 1 + len(detail)
	if m.Options.Preview {
		put(s, 0, previewY, w, "Preview (plain, selected session only; p hides)", accent)
		lines := strings.Split(preview, "\n")
		for i := 0; i < len(lines) && previewY+1+i < h-2; i++ {
			put(s, 0, previewY+1+i, w, lines[i], muted)
		}
	}
	message := m.Message
	if m.Searching {
		message = "Searching: type text, Enter finishes, Escape clears"
	}
	put(s, 0, h-2, w, message, warning)
	attachHint, readOnlyHint := "Enter disabled", "o disabled"
	if choice != nil {
		if choice.Action("attach") == nil {
			attachHint = "Enter attach"
		}
		if choice.Action("read_only_attach") == nil {
			readOnlyHint = "o read-only"
		}
	}
	put(s, 0, h-1, w, "↑↓/jk move | / search | Tab filter | b backend | "+attachHint+" | "+readOnlyHint+" | p preview | r refresh | q quit", muted)
	s.Show()
}
