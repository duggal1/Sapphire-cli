package styles

import "testing"

func TestMarkdownHeadingsUseUnderlineForStructure(t *testing.T) {
	t.Parallel()

	sty := DefaultStyles(false)

	if sty.Markdown.H1.StylePrimitive.Underline == nil || !*sty.Markdown.H1.StylePrimitive.Underline {
		t.Fatal("expected markdown H1 to be underlined")
	}
	if sty.Markdown.H2.StylePrimitive.Underline == nil || !*sty.Markdown.H2.StylePrimitive.Underline {
		t.Fatal("expected markdown H2 to be underlined")
	}
	if sty.PlainMarkdown.H1.StylePrimitive.Underline == nil || !*sty.PlainMarkdown.H1.StylePrimitive.Underline {
		t.Fatal("expected plain markdown H1 to be underlined")
	}
	if sty.PlainMarkdown.H2.StylePrimitive.Underline == nil || !*sty.PlainMarkdown.H2.StylePrimitive.Underline {
		t.Fatal("expected plain markdown H2 to be underlined")
	}
}
