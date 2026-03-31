package list

import (
	"sort"
	"strings"
)

// List represents a list of items that can be lazily rendered. A list is
// always rendered like a chat conversation where items are stacked vertically
// from top to bottom.
type List struct {
	// Viewport size
	width, height int

	// Items in the list
	items []Item

	// Gap between items (0 or less means no gap)
	gap int

	// show list in reverse order
	reverse bool

	// Focus and selection state
	focused     bool
	selectedIdx int // The current selected index -1 means no selection

	// offsetIdx is the index of the first visible item in the viewport.
	offsetIdx int
	// offsetLine is the number of lines of the item at offsetIdx that are
	// scrolled out of view (above the viewport).
	// It must always be >= 0.
	offsetLine int

	// renderCallbacks is a list of callbacks to apply when rendering items.
	renderCallbacks []func(idx, selectedIdx int, item Item) Item

	// Cached heights and prefix sums for fast scrolling.
	heightCache     []int
	heightValid     []bool
	renderCache     []renderedItem
	renderValid     []bool
	prefixHeights   []int
	prefixValidUpTo int
	cacheWidth      int
}

// renderedItem holds the rendered content and height of an item.
type renderedItem struct {
	content string
	height  int
	lines   []string
}

func (l *List) blockHeight(idx int) int {
	if idx < 0 || idx >= len(l.items) {
		return 0
	}
	height := l.itemHeight(idx)
	if l.gap > 0 && idx < len(l.items)-1 {
		height += l.gap
	}
	return height
}

func (l *List) invalidateSelectionChange(previous, next int) {
	if previous >= 0 {
		l.InvalidateItem(previous)
	}
	if next >= 0 && next != previous {
		l.InvalidateItem(next)
	}
}

func (l *List) setSelectedIndex(index int) bool {
	previous := l.selectedIdx
	if index < 0 || index >= len(l.items) {
		l.selectedIdx = -1
	} else {
		l.selectedIdx = index
	}
	if previous == l.selectedIdx {
		return false
	}
	l.invalidateSelectionChange(previous, l.selectedIdx)
	return true
}

func (l *List) ensureCacheSize() {
	if len(l.heightCache) == len(l.items) && len(l.heightValid) == len(l.items) && len(l.renderCache) == len(l.items) && len(l.renderValid) == len(l.items) && len(l.prefixHeights) == len(l.items) {
		return
	}
	oldLen := len(l.heightCache)
	newLen := len(l.items)
	heightCache := make([]int, newLen)
	heightValid := make([]bool, newLen)
	renderCache := make([]renderedItem, newLen)
	renderValid := make([]bool, newLen)
	prefixHeights := make([]int, newLen)
	if oldLen > 0 {
		copy(heightCache, l.heightCache)
		copy(heightValid, l.heightValid)
		copy(renderCache, l.renderCache)
		copy(renderValid, l.renderValid)
		copy(prefixHeights, l.prefixHeights)
	}
	l.heightCache = heightCache
	l.heightValid = heightValid
	l.renderCache = renderCache
	l.renderValid = renderValid
	l.prefixHeights = prefixHeights
	if newLen == 0 {
		l.prefixValidUpTo = -1
		return
	}
	if l.prefixValidUpTo >= newLen {
		l.prefixValidUpTo = newLen - 1
	}
}

func (l *List) setHeightCache(idx int, height int) {
	if idx < 0 || idx >= len(l.items) {
		return
	}
	l.ensureCacheSize()
	l.heightCache[idx] = height
	l.heightValid[idx] = true
	if l.prefixValidUpTo >= idx {
		l.prefixValidUpTo = idx - 1
	}
}

func (l *List) itemHeight(idx int) int {
	if idx < 0 || idx >= len(l.items) {
		return 0
	}
	if l.cacheWidth != l.width {
		l.InvalidateAll()
	}
	l.ensureCacheSize()
	if l.renderValid[idx] {
		return l.renderCache[idx].height
	}

	item := l.items[idx]
	if len(l.renderCallbacks) > 0 {
		for _, cb := range l.renderCallbacks {
			if it := cb(idx, l.selectedIdx, item); it != nil {
				item = it
			}
		}
	}
	rendered := item.Render(l.width)
	rendered = strings.TrimRight(rendered, "\n")
	lines := strings.Split(rendered, "\n")
	height := len(lines)
	ri := renderedItem{content: rendered, height: height, lines: lines}
	l.renderCache[idx] = ri
	l.renderValid[idx] = true
	l.heightCache[idx] = height
	l.heightValid[idx] = true
	return height
}

func (l *List) ensurePrefixUpTo(idx int) {
	if len(l.items) == 0 {
		l.prefixValidUpTo = -1
		return
	}
	if idx >= len(l.items) {
		idx = len(l.items) - 1
	}
	if l.cacheWidth != l.width {
		l.InvalidateAll()
	}
	l.ensureCacheSize()
	if l.prefixValidUpTo >= idx {
		return
	}
	for i := l.prefixValidUpTo + 1; i <= idx; i++ {
		height := l.itemHeight(i)
		blockHeight := height
		if l.gap > 0 && i < len(l.items)-1 {
			blockHeight += l.gap
		}
		if i == 0 {
			l.prefixHeights[i] = blockHeight
		} else {
			l.prefixHeights[i] = l.prefixHeights[i-1] + blockHeight
		}
	}
	l.prefixValidUpTo = idx
}

func (l *List) totalHeight() int {
	if len(l.items) == 0 {
		return 0
	}
	l.ensurePrefixUpTo(len(l.items) - 1)
	return l.prefixHeights[len(l.items)-1]
}

func (l *List) heightBeforeIndex(idx int) int {
	if idx <= 0 {
		return 0
	}
	l.ensurePrefixUpTo(idx - 1)
	return l.prefixHeights[idx-1]
}

func (l *List) findIndexForLine(line int) (int, int) {
	if len(l.items) == 0 {
		return 0, 0
	}
	if line <= 0 {
		return 0, 0
	}
	l.ensurePrefixUpTo(len(l.items) - 1)
	idx := sort.Search(len(l.items), func(i int) bool {
		return l.prefixHeights[i] > line
	})
	if idx >= len(l.items) {
		idx = len(l.items) - 1
	}
	heightBefore := 0
	if idx > 0 {
		heightBefore = l.prefixHeights[idx-1]
	}
	offset := line - heightBefore
	if offset < 0 {
		offset = 0
	}
	return idx, offset
}

// NewList creates a new lazy-loaded list.
func NewList(items ...Item) *List {
	l := new(List)
	l.items = items
	l.selectedIdx = -1
	l.prefixValidUpTo = -1
	return l
}

// RenderCallback defines a function that can modify an item before it is
// rendered.
type RenderCallback func(idx, selectedIdx int, item Item) Item

// RegisterRenderCallback registers a callback to be called when rendering
// items. This can be used to modify items before they are rendered.
func (l *List) RegisterRenderCallback(cb RenderCallback) {
	l.renderCallbacks = append(l.renderCallbacks, cb)
}

// SetSize sets the size of the list viewport.
func (l *List) SetSize(width, height int) {
	l.width = width
	l.height = height
	if l.cacheWidth != width {
		l.InvalidateAll()
	}
}

// SetGap sets the gap between items.
func (l *List) SetGap(gap int) {
	l.gap = gap
	l.InvalidateAll()
}

// Gap returns the gap between items.
func (l *List) Gap() int {
	return l.gap
}

// AtBottom returns whether the list is showing the last item at the bottom.
func (l *List) AtBottom() bool {
	if len(l.items) == 0 {
		return true
	}
	if l.height <= 0 {
		return true
	}

	currentIdx := max(l.offsetIdx, 0)
	currentOffset := max(l.offsetLine, 0)
	linesRemaining := l.height

	for currentIdx < len(l.items) {
		blockHeight := l.blockHeight(currentIdx)
		if blockHeight <= 0 {
			currentIdx++
			currentOffset = 0
			continue
		}
		if currentOffset >= blockHeight {
			currentOffset -= blockHeight
			currentIdx++
			continue
		}

		visible := blockHeight - currentOffset
		if visible > linesRemaining {
			return false
		}

		linesRemaining -= visible
		currentIdx++
		currentOffset = 0
		if linesRemaining == 0 {
			return currentIdx >= len(l.items)
		}
	}

	return true
}

// SetReverse shows the list in reverse order.
func (l *List) SetReverse(reverse bool) {
	l.reverse = reverse
}

// Width returns the width of the list viewport.
func (l *List) Width() int {
	return l.width
}

// Height returns the height of the list viewport.
func (l *List) Height() int {
	return l.height
}

// Len returns the number of items in the list.
func (l *List) Len() int {
	return len(l.items)
}

// lastOffsetItem returns the index and line offsets of the last item that can
// be partially visible in the viewport.
func (l *List) lastOffsetItem() (int, int, int) {
	if len(l.items) == 0 {
		return 0, 0, 0
	}
	if l.height <= 0 {
		return len(l.items) - 1, 0, 0
	}

	linesRemaining := l.height
	for idx := len(l.items) - 1; idx >= 0; idx-- {
		itemHeight := l.itemHeight(idx)
		if itemHeight <= 0 {
			continue
		}
		if linesRemaining <= itemHeight {
			return idx, max(itemHeight-linesRemaining, 0), 0
		}
		linesRemaining -= itemHeight

		if l.gap > 0 && idx > 0 {
			if linesRemaining <= l.gap {
				prevHeight := l.itemHeight(idx - 1)
				return idx - 1, prevHeight + (l.gap - linesRemaining), 0
			}
			linesRemaining -= l.gap
		}
	}

	return 0, 0, 0
}

// getItem renders (if needed) and returns the item at the given index.
func (l *List) getItem(idx int) renderedItem {
	if idx < 0 || idx >= len(l.items) {
		return renderedItem{}
	}

	if l.cacheWidth != l.width {
		l.InvalidateAll()
	}
	l.ensureCacheSize()
	if l.renderValid[idx] {
		return l.renderCache[idx]
	}

	item := l.items[idx]
	if len(l.renderCallbacks) > 0 {
		for _, cb := range l.renderCallbacks {
			if it := cb(idx, l.selectedIdx, item); it != nil {
				item = it
			}
		}
	}

	rendered := item.Render(l.width)
	rendered = strings.TrimRight(rendered, "\n")
	lines := strings.Split(rendered, "\n")
	height := len(lines)
	ri := renderedItem{
		content: rendered,
		height:  height,
		lines:   lines,
	}
	l.renderCache[idx] = ri
	l.renderValid[idx] = true
	l.setHeightCache(idx, height)
	return ri
}

// ScrollToIndex scrolls the list to the given item index.
func (l *List) ScrollToIndex(index int) {
	if index < 0 {
		index = 0
	}
	if index >= len(l.items) {
		index = len(l.items) - 1
	}
	l.offsetIdx = index
	l.offsetLine = 0
}

// ScrollBy scrolls the list by the given number of lines.
func (l *List) ScrollBy(lines int) {
	if len(l.items) == 0 || lines == 0 {
		return
	}

	if l.reverse {
		lines = -lines
	}

	if l.reverse {
		// Fallback to the original behavior for reverse lists.
		if lines > 0 {
			if l.AtBottom() {
				return
			}
			l.offsetLine += lines
			currentItem := l.getItem(l.offsetIdx)
			for l.offsetLine >= currentItem.height {
				l.offsetLine -= currentItem.height
				if l.gap > 0 {
					l.offsetLine = max(0, l.offsetLine-l.gap)
				}
				l.offsetIdx++
				if l.offsetIdx > len(l.items)-1 {
					l.ScrollToBottom()
					return
				}
				currentItem = l.getItem(l.offsetIdx)
			}
			lastOffsetIdx, lastOffsetLine, _ := l.lastOffsetItem()
			if l.offsetIdx > lastOffsetIdx || (l.offsetIdx == lastOffsetIdx && l.offsetLine > lastOffsetLine) {
				l.offsetIdx = lastOffsetIdx
				l.offsetLine = lastOffsetLine
			}
		} else if lines < 0 {
			l.offsetLine += lines
			for l.offsetLine < 0 {
				l.offsetIdx--
				if l.offsetIdx < 0 {
					l.ScrollToTop()
					break
				}
				prevItem := l.getItem(l.offsetIdx)
				totalHeight := prevItem.height
				if l.gap > 0 {
					totalHeight += l.gap
				}
				l.offsetLine += totalHeight
			}
		}
		return
	}

	if lines > 0 {
		linesRemaining := lines
		for linesRemaining > 0 {
			if l.offsetIdx >= len(l.items) {
				l.ScrollToBottom()
				return
			}

			blockHeight := l.blockHeight(l.offsetIdx)
			if blockHeight <= 0 {
				if l.offsetIdx >= len(l.items)-1 {
					l.ScrollToBottom()
					return
				}
				l.offsetIdx++
				l.offsetLine = 0
				continue
			}

			if l.offsetLine >= blockHeight {
				l.offsetLine -= blockHeight
				if l.offsetIdx >= len(l.items)-1 {
					l.ScrollToBottom()
					return
				}
				l.offsetIdx++
				continue
			}

			available := blockHeight - l.offsetLine
			if linesRemaining < available {
				l.offsetLine += linesRemaining
				return
			}

			linesRemaining -= available
			if l.offsetIdx >= len(l.items)-1 {
				l.ScrollToBottom()
				return
			}
			l.offsetIdx++
			l.offsetLine = 0
		}
		return
	}

	linesRemaining := -lines
	for linesRemaining > 0 {
		if l.offsetLine >= linesRemaining {
			l.offsetLine -= linesRemaining
			return
		}

		linesRemaining -= l.offsetLine
		if l.offsetIdx <= 0 {
			l.ScrollToTop()
			return
		}

		l.offsetIdx--
		l.offsetLine = l.blockHeight(l.offsetIdx)
	}
}

// VisibleItemIndices finds the range of items that are visible in the viewport.
// This is used for checking if selected item is in view.
func (l *List) VisibleItemIndices() (startIdx, endIdx int) {
	if len(l.items) == 0 {
		return 0, 0
	}

	startIdx = l.offsetIdx
	currentIdx := startIdx
	visibleHeight := -l.offsetLine

	for currentIdx < len(l.items) {
		item := l.getItem(currentIdx)
		visibleHeight += item.height
		if l.gap > 0 {
			visibleHeight += l.gap
		}

		if visibleHeight >= l.height {
			break
		}
		currentIdx++
	}

	endIdx = currentIdx
	if endIdx >= len(l.items) {
		endIdx = len(l.items) - 1
	}

	return startIdx, endIdx
}

// Render renders the list and returns the visible lines.
func (l *List) Render() string {
	if len(l.items) == 0 {
		return ""
	}

	lines := make([]string, 0, max(l.height, 0))
	currentIdx := l.offsetIdx
	currentOffset := l.offsetLine

	linesNeeded := l.height

	for linesNeeded > 0 && currentIdx < len(l.items) {
		item := l.getItem(currentIdx)
		itemLines := item.lines
		if itemLines == nil {
			itemLines = strings.Split(item.content, "\n")
		}
		itemHeight := len(itemLines)

		if currentOffset >= 0 && currentOffset < itemHeight {
			// Add only the visible content lines needed for this viewport.
			end := min(itemHeight, currentOffset+linesNeeded)
			lines = append(lines, itemLines[currentOffset:end]...)
			linesNeeded = l.height - len(lines)

			if linesNeeded > 0 && l.gap > 0 {
				gapToAdd := min(l.gap, linesNeeded)
				for i := 0; i < gapToAdd; i++ {
					lines = append(lines, "")
				}
				linesNeeded = l.height - len(lines)
			}
		} else {
			// offsetLine starts in the gap
			gapOffset := currentOffset - itemHeight
			gapRemaining := l.gap - gapOffset
			if gapRemaining > 0 {
				gapToAdd := min(gapRemaining, linesNeeded)
				for i := 0; i < gapToAdd; i++ {
					lines = append(lines, "")
				}
			}
		}

		linesNeeded = l.height - len(lines)
		currentIdx++
		currentOffset = 0 // Reset offset for subsequent items
	}

	l.height = max(l.height, 0)

	if len(lines) > l.height {
		lines = lines[:l.height]
	}

	if l.reverse {
		// Reverse the lines so the list renders bottom-to-top.
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	}

	return strings.Join(lines, "\n")
}

// PrependItems prepends items to the list.
func (l *List) PrependItems(items ...Item) {
	l.items = append(items, l.items...)
	l.InvalidateAll()

	// Keep view position relative to the content that was visible
	l.offsetIdx += len(items)

	// Update selection index if valid
	if l.selectedIdx != -1 {
		l.selectedIdx += len(items)
	}
}

// InsertItemsAt inserts items at the given index.
func (l *List) InsertItemsAt(index int, items ...Item) {
	if len(items) == 0 {
		return
	}
	if index < 0 {
		index = 0
	}
	if index > len(l.items) {
		index = len(l.items)
	}

	if index == len(l.items) {
		l.items = append(l.items, items...)
	} else {
		l.items = append(l.items, make([]Item, len(items))...)
		copy(l.items[index+len(items):], l.items[index:len(l.items)-len(items)])
		copy(l.items[index:], items)
	}

	if l.offsetIdx >= index {
		l.offsetIdx += len(items)
	}
	if l.selectedIdx >= index {
		l.selectedIdx += len(items)
	}
	l.InvalidateAll()
}

// SetItems sets the items in the list.
func (l *List) SetItems(items ...Item) {
	l.setItems(true, items...)
}

// setItems sets the items in the list. If evict is true, it clears the
// rendered item cache.
func (l *List) setItems(evict bool, items ...Item) {
	l.items = items
	l.selectedIdx = min(l.selectedIdx, len(l.items)-1)
	l.offsetIdx = min(l.offsetIdx, len(l.items)-1)
	l.offsetLine = 0
	if evict {
		l.InvalidateAll()
	}
}

// AppendItems appends items to the list.
func (l *List) AppendItems(items ...Item) {
	if len(items) == 0 {
		return
	}
	start := len(l.items)
	l.items = append(l.items, items...)
	l.ensureCacheSize()
	for i := start; i < len(l.items); i++ {
		l.heightValid[i] = false
		if i < len(l.renderValid) {
			l.renderValid[i] = false
		}
	}
}

// RemoveItem removes the item at the given index from the list.
func (l *List) RemoveItem(idx int) {
	if idx < 0 || idx >= len(l.items) {
		return
	}

	// Remove the item
	l.items = append(l.items[:idx], l.items[idx+1:]...)

	// Adjust selection if needed
	if l.selectedIdx == idx {
		l.selectedIdx = -1
	} else if l.selectedIdx > idx {
		l.selectedIdx--
	}

	// Adjust offset if needed
	if l.offsetIdx > idx {
		l.offsetIdx--
	} else if l.offsetIdx == idx && l.offsetIdx >= len(l.items) {
		l.offsetIdx = max(0, len(l.items)-1)
		l.offsetLine = 0
	}
	l.InvalidateAll()
}

// InvalidateItem clears cached layout data for a single item.
func (l *List) InvalidateItem(idx int) {
	if idx < 0 {
		return
	}
	l.ensureCacheSize()
	if idx >= len(l.heightValid) {
		return
	}
	l.heightValid[idx] = false
	if idx < len(l.renderValid) {
		l.renderValid[idx] = false
	}
	if l.prefixValidUpTo >= idx {
		l.prefixValidUpTo = idx - 1
	}
}

// InvalidateAll clears all cached layout data.
func (l *List) InvalidateAll() {
	l.heightCache = nil
	l.heightValid = nil
	l.renderCache = nil
	l.renderValid = nil
	l.prefixHeights = nil
	l.prefixValidUpTo = -1
	l.cacheWidth = l.width
}

// Focused returns whether the list is focused.
func (l *List) Focused() bool {
	return l.focused
}

// Focus sets the focus state of the list.
func (l *List) Focus() {
	if l.focused {
		return
	}
	l.focused = true
	l.InvalidateItem(l.selectedIdx)
}

// Blur removes the focus state from the list.
func (l *List) Blur() {
	if !l.focused {
		return
	}
	l.focused = false
	l.InvalidateItem(l.selectedIdx)
}

// ScrollToTop scrolls the list to the top.
func (l *List) ScrollToTop() {
	l.offsetIdx = 0
	l.offsetLine = 0
}

// ScrollToBottom scrolls the list to the bottom.
func (l *List) ScrollToBottom() {
	if len(l.items) == 0 {
		return
	}

	lastOffsetIdx, lastOffsetLine, _ := l.lastOffsetItem()
	l.offsetIdx = lastOffsetIdx
	l.offsetLine = lastOffsetLine
}

// ScrollToSelected scrolls the list to the selected item.
func (l *List) ScrollToSelected() {
	if l.selectedIdx < 0 || l.selectedIdx >= len(l.items) {
		return
	}

	startIdx, endIdx := l.VisibleItemIndices()
	if l.selectedIdx < startIdx {
		// Selected item is above the visible range
		l.offsetIdx = l.selectedIdx
		l.offsetLine = 0
	} else if l.selectedIdx > endIdx {
		// Selected item is below the visible range
		// Scroll so that the selected item is at the bottom
		var totalHeight int
		for i := l.selectedIdx; i >= 0; i-- {
			item := l.getItem(i)
			totalHeight += item.height
			if l.gap > 0 && i < l.selectedIdx {
				totalHeight += l.gap
			}
			if totalHeight >= l.height {
				l.offsetIdx = i
				l.offsetLine = totalHeight - l.height
				break
			}
		}
		if totalHeight < l.height {
			// All items fit in the viewport
			l.ScrollToTop()
		}
	}
}

// SelectedItemInView returns whether the selected item is currently in view.
func (l *List) SelectedItemInView() bool {
	if l.selectedIdx < 0 || l.selectedIdx >= len(l.items) {
		return false
	}
	startIdx, endIdx := l.VisibleItemIndices()
	return l.selectedIdx >= startIdx && l.selectedIdx <= endIdx
}

// SetSelected sets the selected item index in the list.
// It returns -1 if the index is out of bounds.
func (l *List) SetSelected(index int) {
	l.setSelectedIndex(index)
}

// Selected returns the index of the currently selected item. It returns -1 if
// no item is selected.
func (l *List) Selected() int {
	return l.selectedIdx
}

// IsSelectedFirst returns whether the first item is selected.
func (l *List) IsSelectedFirst() bool {
	return l.selectedIdx == 0
}

// IsSelectedLast returns whether the last item is selected.
func (l *List) IsSelectedLast() bool {
	return l.selectedIdx == len(l.items)-1
}

// SelectPrev selects the visually previous item (moves toward visual top).
// It returns whether the selection changed.
func (l *List) SelectPrev() bool {
	if l.reverse {
		// In reverse, visual up = higher index
		if l.selectedIdx < len(l.items)-1 {
			return l.setSelectedIndex(l.selectedIdx + 1)
		}
	} else {
		// Normal: visual up = lower index
		if l.selectedIdx > 0 {
			return l.setSelectedIndex(l.selectedIdx - 1)
		}
	}
	return false
}

// SelectNext selects the next item in the list.
// It returns whether the selection changed.
func (l *List) SelectNext() bool {
	if l.reverse {
		// In reverse, visual down = lower index
		if l.selectedIdx > 0 {
			return l.setSelectedIndex(l.selectedIdx - 1)
		}
	} else {
		// Normal: visual down = higher index
		if l.selectedIdx < len(l.items)-1 {
			return l.setSelectedIndex(l.selectedIdx + 1)
		}
	}
	return false
}

// SelectFirst selects the first item in the list.
// It returns whether the selection changed.
func (l *List) SelectFirst() bool {
	if len(l.items) == 0 {
		return false
	}
	return l.setSelectedIndex(0)
}

// SelectLast selects the last item in the list (highest index).
// It returns whether the selection changed.
func (l *List) SelectLast() bool {
	if len(l.items) == 0 {
		return false
	}
	return l.setSelectedIndex(len(l.items) - 1)
}

// WrapToStart wraps selection to the visual start (for circular navigation).
// In normal mode, this is index 0. In reverse mode, this is the highest index.
func (l *List) WrapToStart() bool {
	if len(l.items) == 0 {
		return false
	}
	if l.reverse {
		return l.setSelectedIndex(len(l.items) - 1)
	}
	return l.setSelectedIndex(0)
}

// WrapToEnd wraps selection to the visual end (for circular navigation).
// In normal mode, this is the highest index. In reverse mode, this is index 0.
func (l *List) WrapToEnd() bool {
	if len(l.items) == 0 {
		return false
	}
	if l.reverse {
		return l.setSelectedIndex(0)
	}
	return l.setSelectedIndex(len(l.items) - 1)
}

// SelectedItem returns the currently selected item. It may be nil if no item
// is selected.
func (l *List) SelectedItem() Item {
	if l.selectedIdx < 0 || l.selectedIdx >= len(l.items) {
		return nil
	}
	return l.items[l.selectedIdx]
}

// SelectFirstInView selects the first item currently in view.
func (l *List) SelectFirstInView() {
	startIdx, _ := l.VisibleItemIndices()
	l.setSelectedIndex(startIdx)
}

// SelectLastInView selects the last item currently in view.
func (l *List) SelectLastInView() {
	_, endIdx := l.VisibleItemIndices()
	l.setSelectedIndex(endIdx)
}

// ItemAt returns the item at the given index.
func (l *List) ItemAt(index int) Item {
	if index < 0 || index >= len(l.items) {
		return nil
	}
	return l.items[index]
}

// ItemIndexAtPosition returns the item at the given viewport-relative y
// coordinate. Returns the item index and the y offset within that item. It
// returns -1, -1 if no item is found.
func (l *List) ItemIndexAtPosition(x, y int) (itemIdx int, itemY int) {
	return l.findItemAtY(x, y)
}

// findItemAtY finds the item at the given viewport y coordinate.
// Returns the item index and the y offset within that item. It returns -1, -1
// if no item is found.
func (l *List) findItemAtY(_, y int) (itemIdx int, itemY int) {
	if y < 0 || y >= l.height {
		return -1, -1
	}

	// Walk through visible items to find which one contains this y
	currentIdx := l.offsetIdx
	currentLine := -l.offsetLine // Negative because offsetLine is how many lines are hidden

	for currentIdx < len(l.items) && currentLine < l.height {
		item := l.getItem(currentIdx)
		itemEndLine := currentLine + item.height

		// Check if y is within this item's visible range
		if y >= currentLine && y < itemEndLine {
			// Found the item, calculate itemY (offset within the item)
			itemY = y - currentLine
			return currentIdx, itemY
		}

		// Move to next item
		currentLine = itemEndLine
		if l.gap > 0 {
			currentLine += l.gap
		}
		currentIdx++
	}

	return -1, -1
}
