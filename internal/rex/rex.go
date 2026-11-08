// Package rex integrates with the Rex terminal multiplexer: it reports
// status with OSC 7501 and opens files in new panes and tabs.
package rex

import (
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// Available reports whether we run inside a Rex pane with the CLI at hand.
func Available() bool {
	if os.Getenv("REX_SESSION") == "" || os.Getenv("REX_BLOCK") == "" {
		return false
	}
	_, err := exec.LookPath("rex")
	return err == nil
}

// InRex reports whether the terminal is a Rex terminal, which understands
// OSC 7501 even when the CLI isn't on PATH.
func InRex() bool {
	return os.Getenv("REX_BLOCK") != "" || os.Getenv("TERM_PROGRAM") == "rex"
}

// StatusSequence builds an OSC 7501 program status report. Rex expects
// colon-separated key=value pairs, with free text base64-encoded.
func StatusSequence(state, title, msg string) string {
	var b strings.Builder
	b.WriteString("\x1b]7501;id=rexmd:state=" + state + ":app=rexmd")
	if title != "" {
		b.WriteString(":title=" + base64.StdEncoding.EncodeToString([]byte(title)))
	}
	if msg != "" {
		b.WriteString(":msg=" + base64.StdEncoding.EncodeToString([]byte(msg)))
	}
	b.WriteString("\x1b\\")
	return b.String()
}

// Split opens argv in a new pane beside ours.
func Split(dir, direction string, argv ...string) error {
	if !Available() {
		return errors.New("not running in Rex")
	}
	args := []string{"split", "-s", os.Getenv("REX_SESSION"), "-b", os.Getenv("REX_BLOCK"),
		"--split=" + direction, "-c", dir, "--"}
	return run(append(args, argv...))
}

// Tab opens argv in a new tab in our session.
func Tab(label, dir string, argv ...string) error {
	if !Available() {
		return errors.New("not running in Rex")
	}
	args := []string{"window", "new", "-s", os.Getenv("REX_SESSION"), label, "-c", dir, "--"}
	return run(append(args, argv...))
}

func run(args []string) error {
	out, err := exec.Command("rex", args...).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return errors.New(msg)
		}
	}
	return err
}
