package rex

import "testing"

func TestStatusSequence(t *testing.T) {
	t.Run("reports state and app under the rexmd record", func(t *testing.T) {
		got := StatusSequence("idle", "", "")

		if got != "\x1b]7501;id=rexmd:state=idle:app=rexmd\x1b\\" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("base64-encodes the title and message", func(t *testing.T) {
		got := StatusSequence("working", "Doc", "hello")

		if got != "\x1b]7501;id=rexmd:state=working:app=rexmd:title=RG9j:msg=aGVsbG8=\x1b\\" {
			t.Errorf("got %q", got)
		}
	})
}
