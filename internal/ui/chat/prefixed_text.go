package chat

import "strings"

func wrapPrefixedText(text string, width int, initialPrefix string, subsequentPrefix string) []string {
	if width <= 0 {
		return nil
	}

	paragraphs := strings.Split(text, "\n")
	lines := make([]string, 0, len(paragraphs))
	for i, paragraph := range paragraphs {
		if strings.TrimSpace(paragraph) == "" {
			if i == 0 {
				lines = append(lines, initialPrefix)
			} else {
				lines = append(lines, subsequentPrefix)
			}
			continue
		}

		prefix := initialPrefix
		available := max(1, width-len([]rune(prefix)))
		words := strings.Fields(paragraph)
		current := prefix
		currentLen := 0
		for _, word := range words {
			wordLen := len([]rune(word))
			if currentLen == 0 {
				current += word
				currentLen = wordLen
				continue
			}
			if currentLen+1+wordLen > available {
				lines = append(lines, current)
				prefix = subsequentPrefix
				available = max(1, width-len([]rune(prefix)))
				current = prefix + word
				currentLen = wordLen
				continue
			}
			current += " " + word
			currentLen += 1 + wordLen
		}
		lines = append(lines, current)
	}

	return lines
}
