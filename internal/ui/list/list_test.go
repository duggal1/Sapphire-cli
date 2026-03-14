package list

import (
	"strings"
	"testing"
)

type fixedItem struct {
	lines int
}

func (f fixedItem) Render(width int) string {
	_ = width
	if f.lines <= 1 {
		return "x"
	}
	return strings.Repeat("x\n", f.lines-1) + "x"
}

func TestScrollToBottomWithGap(t *testing.T) {
	l := NewList(fixedItem{lines: 2}, fixedItem{lines: 3}, fixedItem{lines: 1})
	l.SetGap(1)
	l.SetSize(80, 4)

	l.ScrollToBottom()
	if l.offsetIdx != 1 || l.offsetLine != 1 {
		t.Fatalf("expected offset (1,1), got (%d,%d)", l.offsetIdx, l.offsetLine)
	}

	l.ScrollBy(-2)
	if l.offsetIdx != 0 || l.offsetLine != 2 {
		t.Fatalf("expected offset (0,2) after scroll up, got (%d,%d)", l.offsetIdx, l.offsetLine)
	}
}

type countingItem struct {
	calls *int
	text  string
}

func (c countingItem) Render(width int) string {
	_ = width
	*c.calls++
	return c.text
}

func TestRenderCachingAvoidsRepeatRender(t *testing.T) {
	calls := 0
	l := NewList(countingItem{calls: &calls, text: "hello"})
	l.SetSize(20, 1)

	_ = l.Render()
	_ = l.Render()

	if calls != 1 {
		t.Fatalf("expected Render to be called once, got %d", calls)
	}
}
