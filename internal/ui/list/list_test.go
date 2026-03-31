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

type focusItem struct {
	label   string
	focused bool
	calls   *int
}

func (f *focusItem) Render(width int) string {
	_ = width
	*f.calls++
	if f.focused {
		return ">" + f.label
	}
	return " " + f.label
}

func (f *focusItem) SetFocused(focused bool) {
	f.focused = focused
}

func TestSelectionChangeInvalidatesFocusedRows(t *testing.T) {
	firstCalls := 0
	secondCalls := 0
	first := &focusItem{label: "first", calls: &firstCalls}
	second := &focusItem{label: "second", calls: &secondCalls}

	l := NewList(first, second)
	l.RegisterRenderCallback(FocusedRenderCallback(l))
	l.SetSize(20, 2)
	l.Focus()
	l.SetSelected(0)

	initial := l.Render()
	if !strings.Contains(initial, ">first") {
		t.Fatalf("expected first item to be focused, got %q", initial)
	}

	l.SelectNext()
	next := l.Render()
	if !strings.Contains(next, ">second") {
		t.Fatalf("expected second item to be focused after selection change, got %q", next)
	}
	if strings.Contains(next, ">first") {
		t.Fatalf("expected first item to lose focus after selection change, got %q", next)
	}
	if firstCalls < 2 || secondCalls < 2 {
		t.Fatalf("expected both items to re-render on selection change, got first=%d second=%d", firstCalls, secondCalls)
	}
}

func TestScrollByUsesViewportLocalTraversal(t *testing.T) {
	calls := 0
	items := make([]Item, 1000)
	for i := range items {
		items[i] = countingItem{calls: &calls, text: "line"}
	}

	l := NewList(items...)
	l.SetSize(80, 5)

	l.ScrollBy(1)

	if l.offsetIdx != 1 || l.offsetLine != 0 {
		t.Fatalf("expected offset (1,0) after scrolling one line, got (%d,%d)", l.offsetIdx, l.offsetLine)
	}
	if calls > 8 {
		t.Fatalf("expected scroll to render only nearby items, got %d renders", calls)
	}
}

func TestAtBottomUsesVisibleSliceOnly(t *testing.T) {
	calls := 0
	items := make([]Item, 1000)
	for i := range items {
		items[i] = countingItem{calls: &calls, text: "line"}
	}

	l := NewList(items...)
	l.SetSize(80, 5)

	if l.AtBottom() {
		t.Fatal("expected top of long list to not be at bottom")
	}
	if calls > 8 {
		t.Fatalf("expected AtBottom to inspect only the viewport, got %d renders", calls)
	}
}

func TestScrollToBottomUsesTailTraversal(t *testing.T) {
	calls := 0
	items := make([]Item, 1000)
	for i := range items {
		items[i] = countingItem{calls: &calls, text: "line"}
	}

	l := NewList(items...)
	l.SetSize(80, 5)

	l.ScrollToBottom()

	if !l.AtBottom() {
		t.Fatal("expected list to be at bottom after ScrollToBottom")
	}
	if calls > 12 {
		t.Fatalf("expected ScrollToBottom to render only tail items, got %d renders", calls)
	}
}
