package shell

import (
	"bytes"
	"strings"
	"sync"
)

// LineRingBuffer stores the last N lines of output, providing cursor-based
// reads for "new output since last check". It implements io.Writer.
type LineRingBuffer struct {
	mu           sync.RWMutex
	lines        []string
	head         int
	tail         int
	count        int
	capacity     int
	totalWritten int // Absolute line number index
	partialBuf   bytes.Buffer
}

// NewLineRingBuffer creates a ring buffer holding up to `capacity` lines.
func NewLineRingBuffer(capacity int) *LineRingBuffer {
	if capacity <= 0 {
		capacity = 10000 // Default 10k lines
	}
	return &LineRingBuffer{
		lines:    make([]string, capacity),
		capacity: capacity,
	}
}

// Write implements io.Writer. Separates chunk into lines.
func (r *LineRingBuffer) Write(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	n = len(p)
	
	r.partialBuf.Write(p)
	
	data := r.partialBuf.Bytes()
	for {
		idx := bytes.IndexByte(data, '\n')
		if idx == -1 {
			// No complete line found
			break
		}
		
		lineStr := string(data[:idx]) // Does not include the newline
		r.pushLine(lineStr)
		data = data[idx+1:]
	}
	
	// Retain rest as partial
	r.partialBuf.Reset()
	r.partialBuf.Write(data)

	return n, nil
}

// WriteString implements io.StringWriter (or just a helper)
func (r *LineRingBuffer) WriteString(s string) (n int, err error) {
	return r.Write([]byte(s))
}

func (r *LineRingBuffer) pushLine(line string) {
	r.lines[r.head] = line
	r.head = (r.head + 1) % r.capacity
	if r.count < r.capacity {
		r.count++
	} else {
		r.tail = (r.tail + 1) % r.capacity
	}
	r.totalWritten++
}

// Flush ensures any trailing data without a newline gets pushed as its own line.
func (r *LineRingBuffer) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.partialBuf.Len() > 0 {
		r.pushLine(r.partialBuf.String())
		r.partialBuf.Reset()
	}
}

// ReadSince reads all lines available since the absolute line cursor.
// Returns the joined string, the next absolute cursor, and a bool 
// indicating if some lines were missed (truncated).
func (r *LineRingBuffer) ReadSince(cursor int) (string, int, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []string
	missed := false
	
	// 'oldestAvailable' absolute index
	oldestAvailable := r.totalWritten - r.count
	if oldestAvailable < 0 {
		oldestAvailable = 0
	}

	if cursor < oldestAvailable {
		missed = true
		cursor = oldestAvailable
	}
	
	if cursor > r.totalWritten {
		cursor = r.totalWritten
	}
	
	numToRead := r.totalWritten - cursor
	
	if numToRead > 0 {
		// Calculate starting index in the circular buffer
		startOffset := cursor - oldestAvailable
		idx := (r.tail + startOffset) % r.capacity
		
		for i := 0; i < numToRead; i++ {
			result = append(result, r.lines[idx])
			idx = (idx + 1) % r.capacity
		}
	}
	
	// include partial line so we don't hold it back from user seeing progress
	str := strings.Join(result, "\n")
	if r.partialBuf.Len() > 0 {
		if str != "" {
			str += "\n"
		}
		str += r.partialBuf.String()
	}

	return str, r.totalWritten, missed
}

// String returns everything currently in the buffer (up to capacity limit).
func (r *LineRingBuffer) String() string {
	str, _, _ := r.ReadSince(0)
	return str
}
