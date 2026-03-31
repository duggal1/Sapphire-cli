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

func TestMarkdownStructureUsesStyledPrefixesAndLinks(t *testing.T) {
	t.Parallel()

	sty := DefaultStyles(false)

	if sty.Markdown.H2.StylePrimitive.Prefix != "▸ " {
		t.Fatalf("expected markdown H2 prefix, got %q", sty.Markdown.H2.StylePrimitive.Prefix)
	}
	if sty.Markdown.H3.StylePrimitive.Prefix != "◆ " {
		t.Fatalf("expected markdown H3 prefix, got %q", sty.Markdown.H3.StylePrimitive.Prefix)
	}
	if sty.Markdown.H4.StylePrimitive.Prefix != "• " {
		t.Fatalf("expected markdown H4 prefix, got %q", sty.Markdown.H4.StylePrimitive.Prefix)
	}
	if sty.Markdown.BlockQuote.IndentToken == nil || *sty.Markdown.BlockQuote.IndentToken != "│ " {
		t.Fatal("expected markdown blockquote rail indent")
	}
	if sty.Markdown.Link.Color == nil || *sty.Markdown.Link.Color == "" {
		t.Fatal("expected markdown links to have accent color")
	}
	if sty.PlainMarkdown.H2.StylePrimitive.Prefix != "▸ " {
		t.Fatalf("expected plain markdown H2 prefix, got %q", sty.PlainMarkdown.H2.StylePrimitive.Prefix)
	}
	if sty.PlainMarkdown.BlockQuote.IndentToken == nil || *sty.PlainMarkdown.BlockQuote.IndentToken != "│ " {
		t.Fatal("expected plain markdown blockquote rail indent")
	}
}
