package pager

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

type Mouse struct {
	Button int // 0 left, 1 middle, 2 right, 64/65 wheel up/down
	X, Y   int // 0-based cell
	Press  bool
	Motion bool
	Alt    bool // Option on macOS; SGR Meta modifier
}

// Event is a key press (Key is "j", "G", "enter", "ctrl+d", ...) or a mouse
// event.
type Event struct {
	Key   string
	Mouse *Mouse
}

var csiKeys = map[string]string{
	"A": "up", "B": "down", "C": "right", "D": "left", "H": "home", "F": "end", "Z": "shift+tab",
	"1~": "home", "4~": "end", "5~": "pgup", "6~": "pgdown", "3~": "delete", "7~": "home", "8~": "end",
}

var kittyKeys = map[int]string{27: "esc", 13: "enter", 9: "tab", 127: "backspace", 8: "backspace", 32: " "}

// parseInput decodes terminal input. final says no more bytes are coming
// right now, so a trailing ESC is the Escape key rather than the start of a
// sequence. Bytes of an incomplete sequence are returned in rest.
func parseInput(b []byte, final bool) (evs []Event, rest []byte) {
	for len(b) > 0 {
		ev, n := parseOneEvent(b, final)
		if n == 0 {
			return evs, b
		}
		if ev.Key != "" || ev.Mouse != nil {
			evs = append(evs, ev)
		}
		b = b[n:]
	}
	return evs, nil
}

func parseOneEvent(b []byte, final bool) (Event, int) {
	c := b[0]
	switch {
	case c == 0x1b:
		if len(b) == 1 {
			if final {
				return Event{Key: "esc"}, 1
			}
			return Event{}, 0
		}
		switch b[1] {
		case '[':
			return parseCSI(b)
		case 'O':
			if len(b) < 3 {
				return Event{}, 0
			}
			return Event{Key: csiKeys[string(b[2])]}, 3
		case ']', '_', 'P':
			// A terminal reply (OSC, APC, DCS) arriving late; skip it.
			if i := strings.Index(string(b), "\x1b\\"); i >= 0 {
				return Event{}, i + 2
			}
			if i := strings.IndexByte(string(b), 0x07); i >= 0 {
				return Event{}, i + 1
			}
			return Event{}, 0
		}
		return Event{Key: "alt+" + string(b[1])}, 2
	case c == '\r' || c == '\n':
		return Event{Key: "enter"}, 1
	case c == '\t':
		return Event{Key: "tab"}, 1
	case c == 0x7f || c == 0x08:
		return Event{Key: "backspace"}, 1
	case c < 0x20:
		return Event{Key: "ctrl+" + string(rune('a'+c-1))}, 1
	}
	if !utf8.FullRune(b) {
		return Event{}, 0
	}
	r, n := utf8.DecodeRune(b)
	return Event{Key: string(r)}, n
}

func parseCSI(b []byte) (Event, int) {
	i := 2
	for i < len(b) && (b[i] < 0x40 || b[i] > 0x7e) {
		i++
	}
	if i >= len(b) {
		return Event{}, 0
	}
	params, final := string(b[2:i]), b[i]
	n := i + 1

	if strings.HasPrefix(params, "<") && (final == 'M' || final == 'm') {
		f := strings.Split(params[1:], ";")
		if len(f) != 3 {
			return Event{}, n
		}
		btn, _ := strconv.Atoi(f[0])
		x, _ := strconv.Atoi(f[1])
		y, _ := strconv.Atoi(f[2])
		return Event{Mouse: &Mouse{Button: btn &^ (4 | 8 | 16 | 32), X: x - 1, Y: y - 1, Press: final == 'M', Motion: btn&32 != 0, Alt: btn&8 != 0}}, n
	}

	if final == 'u' {
		f := strings.Split(params, ";")
		code, _ := strconv.Atoi(strings.Split(f[0], ":")[0])
		mods := 0
		if len(f) > 1 {
			m, _ := strconv.Atoi(strings.Split(f[1], ":")[0])
			mods = m - 1
		}
		name, ok := kittyKeys[code]
		if !ok {
			name = string(rune(code))
		}
		if mods&4 != 0 {
			name = "ctrl+" + name
		}
		if mods&2 != 0 {
			name = "alt+" + name
		}
		if mods&8 != 0 {
			name = "super+" + name
		}
		return Event{Key: name}, n
	}

	key := string(final)
	if final == '~' {
		key = strings.Split(params, ";")[0] + "~"
	}
	name := csiKeys[key]
	if mods := strings.Split(params, ";"); name != "" && len(mods) == 2 && mods[1] == "2" {
		name = "shift+" + name
	}
	return Event{Key: name}, n
}
