package pager

import "testing"

func parseOne(t *testing.T, in string) Event {
	t.Helper()
	evs, rest := parseInput([]byte(in), true)
	if len(evs) != 1 {
		t.Fatalf("got %d events %+v (rest %q), want 1", len(evs), evs, rest)
	}
	return evs[0]
}

func TestParseInput(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"a printable character is its rune", "j", "j"},
		{"an uppercase letter keeps its case", "G", "G"},
		{"carriage return is enter", "\r", "enter"},
		{"tab is tab", "\t", "tab"},
		{"DEL is backspace", "\x7f", "backspace"},
		{"a control byte is ctrl plus its letter", "\x04", "ctrl+d"},
		{"an arrow key is named", "\x1b[B", "down"},
		{"an SS3 arrow key is named", "\x1bOA", "up"},
		{"page down is named", "\x1b[6~", "pgdown"},
		{"shift-tab is named", "\x1b[Z", "shift+tab"},
		{"a lone escape at the end of input is escape", "\x1b", "esc"},
		{"a kitty-protocol escape is escape", "\x1b[27u", "esc"},
		{"a kitty-protocol ctrl key carries the modifier", "\x1b[100;5u", "ctrl+d"},
		{"a kitty-protocol enter is enter", "\x1b[13u", "enter"},
		{"a multibyte rune is one key", "é", "é"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev := parseOne(t, c.in)

			if ev.Key != c.want {
				t.Errorf("got %q, want %q", ev.Key, c.want)
			}
		})
	}

	t.Run("an SGR mouse press reports its button and 0-based cell", func(t *testing.T) {
		ev := parseOne(t, "\x1b[<0;5;3M")

		if ev.Mouse == nil || *ev.Mouse != (Mouse{Button: 0, X: 4, Y: 2, Press: true}) {
			t.Errorf("got %+v", ev.Mouse)
		}
	})

	t.Run("a wheel scroll is a mouse event with button 64 or 65", func(t *testing.T) {
		ev := parseOne(t, "\x1b[<65;1;1M")

		if ev.Mouse == nil || ev.Mouse.Button != 65 {
			t.Errorf("got %+v", ev.Mouse)
		}
	})

	t.Run("an incomplete sequence is kept for the next read", func(t *testing.T) {
		_, rest := parseInput([]byte("\x1b[<0;5"), false)

		if string(rest) != "\x1b[<0;5" {
			t.Errorf("got rest %q", rest)
		}
	})

	t.Run("several keys in one read are all parsed", func(t *testing.T) {
		evs, _ := parseInput([]byte("jk\x1b[A"), true)

		if len(evs) != 3 {
			t.Errorf("got %d events", len(evs))
		}
	})
}
