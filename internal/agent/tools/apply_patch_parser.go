package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// ParseError indicates that the patch could not be parsed.
type ParseError struct {
	Message    string
	LineNumber int
}

func (e *ParseError) Error() string {
	if e.LineNumber > 0 {
		return fmt.Sprintf("invalid patch at line %d: %s", e.LineNumber, e.Message)
	}
	return fmt.Sprintf("invalid patch: %s", e.Message)
}

const (
	beginPatchMarker        = "*** Begin Patch"
	endPatchMarker          = "*** End Patch"
	addFileMarker           = "*** Add File: "
	deleteFileMarker        = "*** Delete File: "
	updateFileMarker        = "*** Update File: "
	moveToMarker            = "*** Move to: "
	eofMarker               = "*** End of File"
	changeContextMarker     = "@@ "
	emptyChangeContextMarker = "@@"
)

// Hunk represents a single parsed hunk from an apply_patch block.
type Hunk struct {
	Target string
	Type   string // "add", "delete", "update"

	// For add
	AddContents string

	// For update
	MovePath string
	Chunks   []UpdateFileChunk
}

type UpdateFileChunk struct {
	ChangeContext *string
	OldLines      []string
	NewLines      []string
	IsEndOfFile   bool
}

// ParsePatch leniently parses an apply_patch argument mimicking Codex's parser.rs
func ParsePatch(patch string) ([]Hunk, error) {
	lines := strings.Split(strings.TrimSpace(patch), "\n")
	
	// Fallback/lenient heredoc stripping matching Codex ParseMode::Lenient
	if len(lines) >= 4 {
		first := lines[0]
		last := lines[len(lines)-1]
		if (first == "<<EOF" || first == "<<'EOF'" || first == "<<\"EOF\"") && strings.HasSuffix(last, "EOF") {
			lines = lines[1 : len(lines)-1]
		}
	}

	if len(lines) < 2 {
		return nil, &ParseError{Message: "The first line of the patch must be '*** Begin Patch'"}
	}

	if strings.TrimSpace(lines[0]) != beginPatchMarker {
		return nil, &ParseError{Message: "The first line of the patch must be '*** Begin Patch'"}
	}
	if strings.TrimSpace(lines[len(lines)-1]) != endPatchMarker {
		return nil, &ParseError{Message: "The last line of the patch must be '*** End Patch'"}
	}

	var hunks []Hunk
	remaining := lines[1 : len(lines)-1]
	lineNo := 2

	for len(remaining) > 0 {
		hunk, parsed, err := parseOneHunk(remaining, lineNo)
		if err != nil {
			return nil, err
		}
		hunks = append(hunks, hunk)
		lineNo += parsed
		remaining = remaining[parsed:]
	}

	return hunks, nil
}

func parseOneHunk(lines []string, startLine int) (Hunk, int, error) {
	first := strings.TrimSpace(lines[0])
	
	if path, ok := strings.CutPrefix(first, addFileMarker); ok {
		var content strings.Builder
		parsed := 1
		for _, l := range lines[1:] {
			if strings.HasPrefix(l, "+") {
				content.WriteString(l[1:])
				content.WriteString("\n")
				parsed++
			} else {
				break
			}
		}
		return Hunk{Type: "add", Target: path, AddContents: content.String()}, parsed, nil
	}

	if path, ok := strings.CutPrefix(first, deleteFileMarker); ok {
		return Hunk{Type: "delete", Target: path}, 1, nil
	}

	if path, ok := strings.CutPrefix(first, updateFileMarker); ok {
		rem := lines[1:]
		parsed := 1
		var movePath string

		if len(rem) > 0 && strings.HasPrefix(rem[0], moveToMarker) {
			movePath = strings.TrimPrefix(rem[0], moveToMarker)
			rem = rem[1:]
			parsed++
		}

		var chunks []UpdateFileChunk
		for len(rem) > 0 {
			if strings.TrimSpace(rem[0]) == "" {
				parsed++
				rem = rem[1:]
				continue
			}
			if strings.HasPrefix(rem[0], "***") {
				break
			}

			chunk, cparsed, err := parseUpdateChunk(rem, startLine+parsed, len(chunks) == 0)
			if err != nil {
				return Hunk{}, 0, err
			}
			chunks = append(chunks, chunk)
			parsed += cparsed
			rem = rem[cparsed:]
		}

		if len(chunks) == 0 {
			return Hunk{}, 0, &ParseError{Message: fmt.Sprintf("Update file hunk for path '%s' is empty", path), LineNumber: startLine}
		}

		return Hunk{Type: "update", Target: path, MovePath: movePath, Chunks: chunks}, parsed, nil
	}

	return Hunk{}, 0, &ParseError{Message: fmt.Sprintf("'%s' is not a valid hunk header", first), LineNumber: startLine}
}

func parseUpdateChunk(lines []string, lineNo int, allowMissingContext bool) (UpdateFileChunk, int, error) {
	if len(lines) == 0 {
		return UpdateFileChunk{}, 0, &ParseError{Message: "Update hunk does not contain any lines", LineNumber: lineNo}
	}

	var ctx *string
	startIdx := 0

	if lines[0] == emptyChangeContextMarker {
		startIdx = 1
	} else if c, ok := strings.CutPrefix(lines[0], changeContextMarker); ok {
		ctx = &c
		startIdx = 1
	} else {
		if !allowMissingContext {
			return UpdateFileChunk{}, 0, &ParseError{Message: fmt.Sprintf("Expected update hunk to start with a @@ context marker, got: '%s'", lines[0]), LineNumber: lineNo}
		}
	}

	if startIdx >= len(lines) {
		return UpdateFileChunk{}, 0, &ParseError{Message: "Update hunk does not contain any lines", LineNumber: lineNo + 1}
	}

	chk := UpdateFileChunk{ChangeContext: ctx}
	parsed := 0

	for _, l := range lines[startIdx:] {
		if l == eofMarker {
			if parsed == 0 {
				return chk, 0, &ParseError{Message: "Update hunk does not contain any lines", LineNumber: lineNo + 1}
			}
			chk.IsEndOfFile = true
			parsed++
			break
		}
		if len(l) == 0 {
			chk.OldLines = append(chk.OldLines, "")
			chk.NewLines = append(chk.NewLines, "")
			parsed++
			continue
		}

		c := l[0]
		rest := l[1:]
		if c == ' ' {
			chk.OldLines = append(chk.OldLines, rest)
			chk.NewLines = append(chk.NewLines, rest)
		} else if c == '+' {
			chk.NewLines = append(chk.NewLines, rest)
		} else if c == '-' {
			chk.OldLines = append(chk.OldLines, rest)
		} else {
			if parsed == 0 {
				return chk, 0, &ParseError{Message: fmt.Sprintf("Unexpected line found in update hunk: '%s'", l), LineNumber: lineNo + 1}
			}
			break
		}
		parsed++
	}

	return chk, startIdx + parsed, nil
}

// seekSequence attempts to find the pattern sequence in lines matching Codex's seek_sequence.rs.
func seekSequence(lines []string, pattern []string, start int, eof bool) (int, bool) {
	if len(pattern) == 0 {
		return start, true
	}
	if len(pattern) > len(lines) {
		return 0, false
	}

	searchStart := start
	if eof && len(lines) >= len(pattern) {
		searchStart = len(lines) - len(pattern)
	}

	limit := len(lines) - len(pattern)

	// 1. Exact match
	for i := searchStart; i <= limit; i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if lines[i+j] != pattern[j] {
				match = false
				break
			}
		}
		if match {
			return i, true
		}
	}

	// 2. Right trim match
	for i := searchStart; i <= limit; i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if strings.TrimRightFunc(lines[i+j], unicode.IsSpace) != strings.TrimRightFunc(pattern[j], unicode.IsSpace) {
				match = false
				break
			}
		}
		if match {
			return i, true
		}
	}

	// 3. Trim match
	for i := searchStart; i <= limit; i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if strings.TrimSpace(lines[i+j]) != strings.TrimSpace(pattern[j]) {
				match = false
				break
			}
		}
		if match {
			return i, true
		}
	}

	// 4. Normalize match (mirroring git apply unicode fuzziness)
	normalize := func(s string) string {
		s = strings.TrimSpace(s)
		var out strings.Builder
		for _, c := range s {
			switch c {
			case '\u2010', '\u2011', '\u2012', '\u2013', '\u2014', '\u2015', '\u2212':
				out.WriteRune('-')
			case '\u2018', '\u2019', '\u201A', '\u201B':
				out.WriteRune('\'')
			case '\u201C', '\u201D', '\u201E', '\u201F':
				out.WriteRune('"')
			case '\u00A0', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006', '\u2007', '\u2008', '\u2009', '\u200A', '\u202F', '\u205F', '\u3000':
				out.WriteRune(' ')
			default:
				out.WriteRune(c)
			}
		}
		return out.String()
	}

	for i := searchStart; i <= limit; i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if normalize(lines[i+j]) != normalize(pattern[j]) {
				match = false
				break
			}
		}
		if match {
			return i, true
		}
	}

	return 0, false
}

// deriveNewContentsFromChunks applies a slice of update chunks to the existing file lines.
func deriveNewContentsFromChunks(originalLines []string, path string, chunks []UpdateFileChunk) ([]string, error) {
	if len(originalLines) > 0 && originalLines[len(originalLines)-1] == "" {
		originalLines = originalLines[:len(originalLines)-1]
	}

	type replacement struct {
		startIdx int
		oldLen   int
		newLines []string
	}

	var replacements []replacement
	lineIdx := 0

	for _, chunk := range chunks {
		if chunk.ChangeContext != nil {
			if idx, ok := seekSequence(originalLines, []string{*chunk.ChangeContext}, lineIdx, false); ok {
				lineIdx = idx + 1
			} else {
				return nil, fmt.Errorf("failed to find context '%s' in %s", *chunk.ChangeContext, path)
			}
		}

		if len(chunk.OldLines) == 0 {
			insertionIdx := len(originalLines)
			if len(originalLines) > 0 && originalLines[len(originalLines)-1] == "" {
				insertionIdx = len(originalLines) - 1
			}
			replacements = append(replacements, replacement{startIdx: insertionIdx, oldLen: 0, newLines: chunk.NewLines})
			continue
		}

		pattern := chunk.OldLines
		newSlice := chunk.NewLines

		idx, ok := seekSequence(originalLines, pattern, lineIdx, chunk.IsEndOfFile)
		
		if !ok && len(pattern) > 0 && pattern[len(pattern)-1] == "" {
			pattern = pattern[:len(pattern)-1]
			if len(newSlice) > 0 && newSlice[len(newSlice)-1] == "" {
				newSlice = newSlice[:len(newSlice)-1]
			}
			idx, ok = seekSequence(originalLines, pattern, lineIdx, chunk.IsEndOfFile)
		}

		if ok {
			replacements = append(replacements, replacement{startIdx: idx, oldLen: len(pattern), newLines: newSlice})
			lineIdx = idx + len(pattern)
		} else {
			return nil, fmt.Errorf("failed to find expected lines in %s:\n%s", path, strings.Join(chunk.OldLines, "\n"))
		}
	}

	// Replace backwards to avoid shifting index bugs
	for i := len(replacements) - 1; i >= 0; i-- {
		rep := replacements[i]

		// Delete
		if rep.oldLen > 0 {
			originalLines = append(originalLines[:rep.startIdx], originalLines[rep.startIdx+rep.oldLen:]...)
		}

		// Insert
		if len(rep.newLines) > 0 {
			newLines := make([]string, len(originalLines)+len(rep.newLines))
			copy(newLines, originalLines[:rep.startIdx])
			copy(newLines[rep.startIdx:], rep.newLines)
			copy(newLines[rep.startIdx+len(rep.newLines):], originalLines[rep.startIdx:])
			originalLines = newLines
		}
	}

	if len(originalLines) == 0 || originalLines[len(originalLines)-1] != "" {
		originalLines = append(originalLines, "")
	}

	return originalLines, nil
}

// ApplyHunks applies the parsed hunks to the file system.
func ApplyHunks(workingDir string, hunks []Hunk) (map[string]string, error) {
	affected := make(map[string]string)

	for _, hunk := range hunks {
		targetPath := filepath.Join(workingDir, hunk.Target)
		
		switch hunk.Type {
		case "add":
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return affected, fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
			}
			if err := os.WriteFile(targetPath, []byte(hunk.AddContents), 0644); err != nil {
				return affected, fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			affected[targetPath] = "added"
		case "delete":
			if err := os.Remove(targetPath); err != nil {
				return affected, fmt.Errorf("failed to delete file %s: %w", targetPath, err)
			}
			affected[targetPath] = "deleted"
		case "update":
			contentBytes, err := os.ReadFile(targetPath)
			if err != nil {
				return affected, fmt.Errorf("failed to read file %s: %w", targetPath, err)
			}
			originalLines := strings.Split(string(contentBytes), "\n")

			newLines, err := deriveNewContentsFromChunks(originalLines, targetPath, hunk.Chunks)
			if err != nil {
				return affected, err
			}

			newContent := strings.Join(newLines, "\n")
			destPath := targetPath
			if hunk.MovePath != "" {
				destPath = filepath.Join(workingDir, hunk.MovePath)
				if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
					return affected, fmt.Errorf("failed to create directory for %s: %w", destPath, err)
				}
				if err := os.Remove(targetPath); err != nil {
					return affected, fmt.Errorf("failed to remove original file %s: %w", targetPath, err)
				}
			}

			if err := os.WriteFile(destPath, []byte(newContent), 0644); err != nil {
				return affected, fmt.Errorf("failed to write file %s: %w", destPath, err)
			}

			affected[destPath] = "modified"
		}
	}

	return affected, nil
}
