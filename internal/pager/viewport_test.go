package pager

import (
	"testing"

	"github.com/ncphillips/rex-md/internal/doc"
)

func TestViewportReservesBottomPadding(t *testing.T) {
	p := &Pager{rows: 24, doc: &doc.Document{Lines: make([]doc.Line, 50)}}
	if got := p.viewRows(); got != 21 {
		t.Fatalf("expected 21 document rows, got %d", got)
	}
	p.top = 50
	p.clampTop()
	if p.top+p.viewRows() != len(p.doc.Lines) {
		t.Fatal("end of document should remain reachable above bottom padding")
	}
}

func TestSmallViewportStillHasContentRow(t *testing.T) {
	for rows := 1; rows <= 4; rows++ {
		p := &Pager{rows: rows}
		if p.viewRows() < 1 {
			t.Fatalf("terminal with %d rows has no content area", rows)
		}
	}
}
