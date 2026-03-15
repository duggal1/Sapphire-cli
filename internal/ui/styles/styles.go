package styles

import (
	"image/color"
	"strings"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/sapphire/internal/ui/diffview"
	"github.com/charmbracelet/x/exp/charmtone"
)

const (
	CheckIcon   string = "✓"
	SpinnerIcon string = "⋯"
	LoadingIcon string = "⟳"
	ModelIcon   string = "◇"

	ArrowRightIcon string = "→"

	ToolPending string = "●"
	ToolSuccess string = "✓"
	ToolError   string = "×"

	RadioOn  string = "◉"
	RadioOff string = "○"

	BorderThin  string = "│"
	BorderThick string = "▌"

	SectionSeparator string = "─"

	TodoCompletedIcon  string = "✓"
	TodoPendingIcon    string = "•"
	TodoInProgressIcon string = "→"

	ImageIcon string = "■"
	TextIcon  string = "≡"

	ScrollbarThumb string = "┃"
	ScrollbarTrack string = "│"

	LSPErrorIcon   string = "E"
	LSPWarningIcon string = "W"
	LSPInfoIcon    string = "I"
	LSPHintIcon    string = "H"
)

const (
	defaultMargin     = 2
	defaultListIndent = 2
)

type Styles struct {
	WindowTooSmall lipgloss.Style

	// Reusable text styles
	Base      lipgloss.Style
	Muted     lipgloss.Style
	HalfMuted lipgloss.Style
	Subtle    lipgloss.Style

	// Tags
	TagBase  lipgloss.Style
	TagError lipgloss.Style
	TagInfo  lipgloss.Style

	// Header
	Header struct {
		Charm        lipgloss.Style // Style for "Charm™" label
		Diagonals    lipgloss.Style // Style for diagonal separators (╱)
		Percentage   lipgloss.Style // Style for context percentage
		Keystroke    lipgloss.Style // Style for keystroke hints (e.g., "ctrl+d")
		KeystrokeTip lipgloss.Style // Style for keystroke action text (e.g., "open", "close")
		WorkingDir   lipgloss.Style // Style for current working directory
		Separator    lipgloss.Style // Style for separator dots (•)
	}

	CompactDetails struct {
		View    lipgloss.Style
		Version lipgloss.Style
		Title   lipgloss.Style
	}

	// Panels
	PanelMuted  lipgloss.Style
	PanelBase   lipgloss.Style
	PanelPadded lipgloss.Style

	// Line numbers for code blocks
	LineNumber lipgloss.Style

	// Message borders
	FocusedMessageBorder lipgloss.Border

	// Tool calls
	ToolCallPending   lipgloss.Style
	ToolCallError     lipgloss.Style
	ToolCallSuccess   lipgloss.Style
	ToolCallCancelled lipgloss.Style
	EarlyStateMessage lipgloss.Style

	// Text selection
	TextSelection lipgloss.Style

	// LSP and MCP status indicators
	ResourceGroupTitle     lipgloss.Style
	ResourceOfflineIcon    lipgloss.Style
	ResourceBusyIcon       lipgloss.Style
	ResourceErrorIcon      lipgloss.Style
	ResourceOnlineIcon     lipgloss.Style
	ResourceName           lipgloss.Style
	ResourceStatus         lipgloss.Style
	ResourceAdditionalText lipgloss.Style

	// Markdown & Chroma
	Markdown      ansi.StyleConfig
	PlainMarkdown ansi.StyleConfig

	// Inputs
	TextInput textinput.Styles
	TextArea  textarea.Styles

	// Help
	Help help.Styles

	// Diff
	Diff diffview.Style

	// FilePicker
	FilePicker filepicker.Styles

	// Buttons
	ButtonFocus lipgloss.Style
	ButtonBlur  lipgloss.Style

	// Borders
	BorderFocus lipgloss.Style
	BorderBlur  lipgloss.Style

	YellowMode bool

	// Editor
	EditorPromptNormalFocused         lipgloss.Style
	EditorPromptNormalBlurred         lipgloss.Style
	EditorPromptYoloIconFocused       lipgloss.Style
	EditorPromptYoloIconBlurred       lipgloss.Style
	EditorPromptYoloDotsFocused       lipgloss.Style
	EditorPromptYoloDotsBlurred       lipgloss.Style
	EditorPromptStatusIconFocused     lipgloss.Style
	EditorPromptStatusIconBlurred     lipgloss.Style
	EditorPromptStatusIconWarnFocused lipgloss.Style
	EditorPromptStatusIconWarnBlurred lipgloss.Style
	EditorPromptStatusDotsWarn        lipgloss.Style
	EditorPromptStatusDotsWarnMuted   lipgloss.Style
	EditorPromptStatusDotsOk          lipgloss.Style
	EditorPromptStatusDotsOkMuted     lipgloss.Style

	// Radio
	RadioOn  lipgloss.Style
	RadioOff lipgloss.Style

	// Background
	Background color.Color

	// Logo
	LogoFieldColor   color.Color
	LogoTitleColorA  color.Color
	LogoTitleColorB  color.Color
	LogoCharmColor   color.Color
	LogoVersionColor color.Color

	// Colors - semantic colors for tool rendering.
	Primary       color.Color
	Secondary     color.Color
	Tertiary      color.Color
	Highlight     color.Color // Warm amber for success/completed states
	BgBase        color.Color
	BgBaseLighter color.Color
	BgSubtle      color.Color
	BgOverlay     color.Color
	FgBase        color.Color
	FgMuted       color.Color
	FgHalfMuted   color.Color
	FgSubtle      color.Color
	Border        color.Color
	BorderColor   color.Color // Border focus color
	Error         color.Color
	Warning       color.Color
	Info          color.Color
	White         color.Color
	BlueLight     color.Color
	Blue          color.Color
	BlueDark      color.Color
	GreenLight    color.Color
	Green         color.Color
	GreenDark     color.Color
	Red           color.Color
	RedDark       color.Color
	Yellow        color.Color

	// Section Title
	Section struct {
		Title lipgloss.Style
		Line  lipgloss.Style
	}

	// Initialize
	Initialize struct {
		Header  lipgloss.Style
		Content lipgloss.Style
		Accent  lipgloss.Style
	}

	// LSP
	LSP struct {
		ErrorDiagnostic   lipgloss.Style
		WarningDiagnostic lipgloss.Style
		HintDiagnostic    lipgloss.Style
		InfoDiagnostic    lipgloss.Style
	}

	// Files
	Files struct {
		Path      lipgloss.Style
		Additions lipgloss.Style
		Deletions lipgloss.Style
	}

	// Chat
	Chat struct {
		// Message item styles
		Message struct {
			UserBlurred      lipgloss.Style
			UserFocused      lipgloss.Style
			AssistantBlurred lipgloss.Style
			AssistantFocused lipgloss.Style
			NoContent        lipgloss.Style
			Thinking         lipgloss.Style
			ErrorTag         lipgloss.Style
			ErrorTitle       lipgloss.Style
			ErrorDetails     lipgloss.Style
			ToolCallFocused  lipgloss.Style
			ToolCallCompact  lipgloss.Style
			ToolCallBlurred  lipgloss.Style
			SectionHeader    lipgloss.Style

			// Thinking section styles
			ThinkingBox            lipgloss.Style // Background for thinking content
			ThinkingTruncationHint lipgloss.Style // "… (N lines hidden)" hint
			ThinkingFooterTitle    lipgloss.Style // "Thought for" text
			ThinkingFooterDuration lipgloss.Style // Duration value
			AssistantInfoIcon      lipgloss.Style
			AssistantInfoModel     lipgloss.Style
			AssistantInfoProvider  lipgloss.Style
			AssistantInfoDuration  lipgloss.Style
		}
	}

	// Tool - styles for tool call rendering
	Tool struct {
		// Icon styles with tool status
		IconPending   lipgloss.Style // Pending operation icon
		IconSuccess   lipgloss.Style // Successful operation icon
		IconError     lipgloss.Style // Error operation icon
		IconCancelled lipgloss.Style // Cancelled operation icon

		// Tool name styles
		NameNormal lipgloss.Style // Normal tool name
		NameNested lipgloss.Style // Nested tool name

		// Parameter list styles
		ParamMain lipgloss.Style // Main parameter
		ParamKey  lipgloss.Style // Parameter keys

		// Content rendering styles
		ContentLine           lipgloss.Style // Individual content line with background and width
		ContentTruncation     lipgloss.Style // Truncation message "… (N lines)"
		ContentCodeLine       lipgloss.Style // Code line with background and width
		ContentCodeTruncation lipgloss.Style // Code truncation message with bgBase
		ContentCodeBg         color.Color    // Background color for syntax highlighting
		Body                  lipgloss.Style // Body content padding (PaddingLeft(2))
		FileBlock             lipgloss.Style // Wrapper for file/code content blocks

		// Deprecated - kept for backward compatibility
		ContentBg         lipgloss.Style // Content background
		ContentText       lipgloss.Style // Content text
		ContentLineNumber lipgloss.Style // Line numbers in code

		// State message styles
		StateWaiting   lipgloss.Style // "Waiting for tool response..."
		StateCancelled lipgloss.Style // "Canceled."

		// Error styles
		ErrorTag     lipgloss.Style // ERROR tag
		ErrorMessage lipgloss.Style // Error message text

		// Diff styles
		DiffTruncation lipgloss.Style // Diff truncation message with padding

		// Multi-edit note styles
		NoteTag     lipgloss.Style // NOTE tag (yellow background)
		NoteMessage lipgloss.Style // Note message text

		// Job header styles (for bash jobs)
		JobIconPending  lipgloss.Style // Pending job icon (green dark)
		JobIconError    lipgloss.Style // Error job icon (red dark)
		JobIconSuccess  lipgloss.Style // Success job icon (green)
		JobToolName     lipgloss.Style // Job tool name "Bash" (blue)
		JobAction       lipgloss.Style // Action text (Start, Output, Kill)
		JobPID          lipgloss.Style // PID text
		JobDescription  lipgloss.Style // Description text
		BashLabel       lipgloss.Style // Bash label for command line
		BashCommand     lipgloss.Style // Bash command line styling
		BashOutputLabel lipgloss.Style // Bash output label

		// Agent task styles
		AgentTaskTag lipgloss.Style // Agent task tag (blue background, bold)
		AgentPrompt  lipgloss.Style // Agent prompt text

		// Agentic fetch styles
		AgenticFetchPromptTag lipgloss.Style // Agentic fetch prompt tag (green background, bold)

		// Todo styles
		TodoRatio          lipgloss.Style // Todo ratio (e.g., "2/5")
		TodoCompletedIcon  lipgloss.Style // Completed todo icon
		TodoInProgressIcon lipgloss.Style // In-progress todo icon
		TodoPendingIcon    lipgloss.Style // Pending todo icon

		// MCP tools
		MCPName     lipgloss.Style // The mcp name
		MCPToolName lipgloss.Style // The mcp tool name
		MCPArrow    lipgloss.Style // The mcp arrow icon

		// Structured search/list styles
		ListRoot      lipgloss.Style
		ListDirectory lipgloss.Style
		ListFile      lipgloss.Style
		ListMeta      lipgloss.Style
		ListHint      lipgloss.Style
		GrepFile      lipgloss.Style
		GrepLine      lipgloss.Style
		GrepMatch     lipgloss.Style
		GrepContext   lipgloss.Style

		// Images and external resources
		ResourceLoadedText      lipgloss.Style
		ResourceLoadedIndicator lipgloss.Style
		ResourceName            lipgloss.Style
		ResourceSize            lipgloss.Style
		MediaType               lipgloss.Style
		SkillTag                lipgloss.Style
	}

	// Dialog styles
	Dialog struct {
		Title       lipgloss.Style
		TitleText   lipgloss.Style
		TitleError  lipgloss.Style
		TitleAccent lipgloss.Style
		// View is the main content area style.
		View          lipgloss.Style
		PrimaryText   lipgloss.Style
		SecondaryText lipgloss.Style
		// HelpView is the line that contains the help.
		HelpView lipgloss.Style
		Help     struct {
			Ellipsis       lipgloss.Style
			ShortKey       lipgloss.Style
			ShortDesc      lipgloss.Style
			ShortSeparator lipgloss.Style
			FullKey        lipgloss.Style
			FullDesc       lipgloss.Style
			FullSeparator  lipgloss.Style
		}

		NormalItem   lipgloss.Style
		SelectedItem lipgloss.Style
		InputPrompt  lipgloss.Style

		List lipgloss.Style

		Spinner lipgloss.Style

		// ContentPanel is used for content blocks with subtle background.
		ContentPanel lipgloss.Style

		// Scrollbar styles for scrollable content.
		ScrollbarThumb lipgloss.Style
		ScrollbarTrack lipgloss.Style

		// Arguments
		Arguments struct {
			Content                  lipgloss.Style
			Description              lipgloss.Style
			InputLabelBlurred        lipgloss.Style
			InputLabelFocused        lipgloss.Style
			InputRequiredMarkBlurred lipgloss.Style
			InputRequiredMarkFocused lipgloss.Style
		}

		Commands struct{}

		ImagePreview lipgloss.Style

		Sessions struct {
			// styles for when we are in delete mode
			DeletingView                   lipgloss.Style
			DeletingItemFocused            lipgloss.Style
			DeletingItemBlurred            lipgloss.Style
			DeletingTitle                  lipgloss.Style
			DeletingMessage                lipgloss.Style
			DeletingTitleGradientFromColor color.Color
			DeletingTitleGradientToColor   color.Color

			// styles for when we are in update mode
			RenamingView                   lipgloss.Style
			RenamingingItemFocused         lipgloss.Style
			RenamingItemBlurred            lipgloss.Style
			RenamingingTitle               lipgloss.Style
			RenamingingMessage             lipgloss.Style
			RenamingTitleGradientFromColor color.Color
			RenamingTitleGradientToColor   color.Color
			RenamingPlaceholder            lipgloss.Style
		}
	}

	// Status bar and help
	Status struct {
		Help lipgloss.Style

		ErrorIndicator   lipgloss.Style
		WarnIndicator    lipgloss.Style
		InfoIndicator    lipgloss.Style
		UpdateIndicator  lipgloss.Style
		SuccessIndicator lipgloss.Style

		ErrorMessage   lipgloss.Style
		WarnMessage    lipgloss.Style
		InfoMessage    lipgloss.Style
		UpdateMessage  lipgloss.Style
		SuccessMessage lipgloss.Style
	}

	Toast struct {
		SuccessColor color.Color
		ErrorColor   color.Color
		WarnColor    color.Color
		InfoColor    color.Color
		TextColor    color.Color
	}

	// Completions popup styles
	Completions struct {
		Normal  lipgloss.Style
		Focused lipgloss.Style
		Match   lipgloss.Style
	}

	// Attachments styles
	Attachments struct {
		Normal               lipgloss.Style
		Image                lipgloss.Style
		Text                 lipgloss.Style
		Deleting             lipgloss.Style
		PasteBlock           lipgloss.Style
		PasteSelected        lipgloss.Style
		PastePalette         []color.Color
		PasteSelectedPalette []color.Color
	}

	// Pills styles for todo/queue pills
	Pills struct {
		Base            lipgloss.Style // Base pill style with padding
		Focused         lipgloss.Style // Focused pill with visible border
		Blurred         lipgloss.Style // Blurred pill with hidden border
		QueueItemPrefix lipgloss.Style // Prefix for queue list items
		HelpKey         lipgloss.Style // Keystroke hint style
		HelpText        lipgloss.Style // Help action text style
		Area            lipgloss.Style // Pills area container
		TodoSpinner     lipgloss.Style // Todo spinner style
	}
}

// ChromaTheme converts the current markdown chroma styles to a chroma
// StyleEntries map.
func (s *Styles) ChromaTheme() chroma.StyleEntries {
	rules := s.Markdown.CodeBlock

	return chroma.StyleEntries{
		chroma.Text:                chromaStyle(rules.Chroma.Text),
		chroma.Error:               chromaStyle(rules.Chroma.Error),
		chroma.Comment:             chromaStyle(rules.Chroma.Comment),
		chroma.CommentPreproc:      chromaStyle(rules.Chroma.CommentPreproc),
		chroma.Keyword:             chromaStyle(rules.Chroma.Keyword),
		chroma.KeywordReserved:     chromaStyle(rules.Chroma.KeywordReserved),
		chroma.KeywordNamespace:    chromaStyle(rules.Chroma.KeywordNamespace),
		chroma.KeywordType:         chromaStyle(rules.Chroma.KeywordType),
		chroma.Operator:            chromaStyle(rules.Chroma.Operator),
		chroma.Punctuation:         chromaStyle(rules.Chroma.Punctuation),
		chroma.Name:                chromaStyle(rules.Chroma.Name),
		chroma.NameBuiltin:         chromaStyle(rules.Chroma.NameBuiltin),
		chroma.NameTag:             chromaStyle(rules.Chroma.NameTag),
		chroma.NameAttribute:       chromaStyle(rules.Chroma.NameAttribute),
		chroma.NameClass:           chromaStyle(rules.Chroma.NameClass),
		chroma.NameConstant:        chromaStyle(rules.Chroma.NameConstant),
		chroma.NameDecorator:       chromaStyle(rules.Chroma.NameDecorator),
		chroma.NameException:       chromaStyle(rules.Chroma.NameException),
		chroma.NameFunction:        chromaStyle(rules.Chroma.NameFunction),
		chroma.NameOther:           chromaStyle(rules.Chroma.NameOther),
		chroma.Literal:             chromaStyle(rules.Chroma.Literal),
		chroma.LiteralNumber:       chromaStyle(rules.Chroma.LiteralNumber),
		chroma.LiteralDate:         chromaStyle(rules.Chroma.LiteralDate),
		chroma.LiteralString:       chromaStyle(rules.Chroma.LiteralString),
		chroma.LiteralStringEscape: chromaStyle(rules.Chroma.LiteralStringEscape),
		chroma.GenericDeleted:      chromaStyle(rules.Chroma.GenericDeleted),
		chroma.GenericEmph:         chromaStyle(rules.Chroma.GenericEmph),
		chroma.GenericInserted:     chromaStyle(rules.Chroma.GenericInserted),
		chroma.GenericStrong:       chromaStyle(rules.Chroma.GenericStrong),
		chroma.GenericSubheading:   chromaStyle(rules.Chroma.GenericSubheading),
		chroma.Background:          chromaStyle(rules.Chroma.Background),
	}
}

// DialogHelpStyles returns the styles for dialog help.
func (s *Styles) DialogHelpStyles() help.Styles {
	return help.Styles(s.Dialog.Help)
}

// Yellow mode accent colors for alternate theme.
var (
	yellowPrimaryHex   = "#FFD700"
	yellowSecondaryHex = "#FFC107"
	yellowTertiaryHex  = "#B8860B"
	yellowPrimary      = lipgloss.Color(yellowPrimaryHex)
	yellowSecondary    = lipgloss.Color(yellowSecondaryHex)
	yellowTertiary     = lipgloss.Color(yellowTertiaryHex)
)

// DefaultStyles returns the default styles for the UI.
func DefaultStyles(yellowMode bool) Styles {
	var (
		// Semantic Theme Colors - refined warm orange palette
		primaryHex   = "#F08A24"
		primary      = lipgloss.Color(primaryHex)
		secondaryHex = "#D98945"
		secondary    = lipgloss.Color(secondaryHex)
		highlightHex = "#F5C15A"
		highlight    = lipgloss.Color(highlightHex)
		tertiaryHex  = "#8B5A2B"
		tertiary     = lipgloss.Color(tertiaryHex)
		// Markdown accent pair (primary orange + complementary accent)
		markdownSecondaryHex = "#A855F7" // Purple for secondary headings
		markdownTertiaryHex  = "#F97316" // Orange for tertiary/accents

		// Backgrounds
		bgBase        = lipgloss.Color("#15110f")
		bgSubtle      = lipgloss.Color("#171311")
		bgOverlay     = lipgloss.Color("#181412")
		bgBaseLighter = lipgloss.Color("#191513")

		thinkingBg     = bgOverlay
		thinkingBorder = lipgloss.Color("#2a1d16")


		// Foregrounds
		fgBase      = charmtone.Ash
		fgMuted     = charmtone.Squid
		fgHalfMuted = charmtone.Smoke
		fgSubtle    = charmtone.Oyster

		// Borders
		border      = lipgloss.Color("#3a2c24ff")
		borderFocus = lipgloss.Color("#56453bff")

		// Status palette
		error   = lipgloss.Color("#FB7185")
		warning = charmtone.Zest
		info    = charmtone.Sardine

		// Toast backgrounds (status bar)
		toastSuccessBg     = lipgloss.Color("#00aa44ff")
		toastSuccessBorder = lipgloss.Color("#166534")
		toastInfoBg        = lipgloss.Color("#14532D")
		toastInfoBorder    = lipgloss.Color("#166534")
		toastUpdateBg      = lipgloss.Color("#14532D")
		toastUpdateBorder  = lipgloss.Color("#166534")
		toastWarnBg        = lipgloss.Color("#b14646ff")
		toastWarnBorder    = lipgloss.Color("#B91C1C")
		toastErrorBg       = lipgloss.Color("#991B1B")
		toastErrorBorder   = lipgloss.Color("#B91C1C")

		// Toast overlay colors
		toastSuccessColor = charmtone.Guac
		toastWarnColor    = warning
		toastErrorColor   = error
		toastInfoColor    = secondary
		toastTextColor    = charmtone.Butter

		// Colors
		white = charmtone.Butter

		// Warm neutral accents
		blueLight = charmtone.Sardine
		blue      = charmtone.Sardine
		blueDark  = charmtone.Sardine

		yellowHex = "#F59E0B"
		yellow    = lipgloss.Color(yellowHex)

		// Bright green for todo ticks and success states
		greenLight = lipgloss.Color("#4ADE80") // Bright green (Tailwind green-400)
		green      = charmtone.Guac
		greenDark  = charmtone.Guac

		red     = lipgloss.Color("#ff4f6fff")
		redDark = charmtone.Sriracha
	)

	if yellowMode {
		primary = yellowPrimary
		secondary = yellowSecondary
		tertiary = yellowTertiary
		primaryHex = yellowPrimaryHex
		secondaryHex = yellowSecondaryHex
		tertiaryHex = yellowTertiaryHex
	}

	normalBorder := lipgloss.NormalBorder()

	base := lipgloss.NewStyle().Foreground(fgBase)

	s := Styles{}

	s.Background = bgBase

	// Populate color fields
	s.Primary = primary
	s.Secondary = secondary
	s.Tertiary = tertiary
	s.Highlight = highlight
	s.BgBase = bgBase
	s.BgBaseLighter = bgBaseLighter
	s.BgSubtle = bgSubtle
	s.BgOverlay = bgOverlay
	s.FgBase = fgBase
	s.FgMuted = fgMuted
	s.FgHalfMuted = fgHalfMuted
	s.FgSubtle = fgSubtle
	s.Border = border
	s.BorderColor = borderFocus
	s.Error = error
	s.Warning = warning
	s.Info = info
	s.White = white
	s.YellowMode = yellowMode
	s.BlueLight = blueLight
	s.Blue = blue
	s.BlueDark = blueDark
	s.GreenLight = greenLight
	s.Green = green
	s.GreenDark = greenDark
	s.Red = red
	s.RedDark = redDark
	s.Yellow = yellow
	s.Toast.SuccessColor = toastSuccessColor
	s.Toast.WarnColor = toastWarnColor
	s.Toast.ErrorColor = toastErrorColor
	s.Toast.InfoColor = toastInfoColor
	s.Toast.TextColor = toastTextColor

	s.TextInput = textinput.Styles{
		Focused: textinput.StyleState{
			Text:        base,
			Placeholder: base.Foreground(fgSubtle),
			Prompt:      base.Foreground(tertiary),
			Suggestion:  base.Foreground(fgSubtle),
		},
		Blurred: textinput.StyleState{
			Text:        base.Foreground(fgMuted),
			Placeholder: base.Foreground(fgSubtle),
			Prompt:      base.Foreground(fgMuted),
			Suggestion:  base.Foreground(fgSubtle),
		},
		Cursor: textinput.CursorStyle{
			Color: secondary,
			Shape: tea.CursorBlock,
			Blink: true,
		},
	}

	s.TextArea = textarea.Styles{
		Focused: textarea.StyleState{
			Base:             base.PaddingTop(0).PaddingBottom(0),
			Text:             base,
			LineNumber:       base.Foreground(fgSubtle),
			CursorLine:       base,
			CursorLineNumber: base.Foreground(fgSubtle),
			Placeholder:      base.Foreground(fgSubtle),
			Prompt:           base.Foreground(tertiary),
			EndOfBuffer:      base.Foreground(fgSubtle),
		},
		Blurred: textarea.StyleState{
			Base:             base.PaddingTop(0).PaddingBottom(0),
			Text:             base.Foreground(fgMuted),
			LineNumber:       base.Foreground(fgMuted),
			CursorLine:       base,
			CursorLineNumber: base.Foreground(fgMuted),
			Placeholder:      base.Foreground(fgSubtle),
			Prompt:           base.Foreground(fgMuted),
			EndOfBuffer:      base.Foreground(fgSubtle),
		},
		Cursor: textarea.CursorStyle{
			Color: secondary,
			Shape: tea.CursorBlock,
			Blink: true,
		},
	}

	syntaxGray := "#D4D0CB"
	syntaxGrayMuted := "#8E8379"
	syntaxOrange := "#F59E0B"    // Bright orange for strings and escapes
	syntaxPurple := "#C084FC"    // Vibrant purple for keywords
	syntaxBlue := "#7BA2F7"
	syntaxPink := "#FB7185"      // Bright pink for functions
	syntaxGreen := "#4ADE80"     // Bright green for constants
	syntaxLime := "#A3E635"      // Vibrant lime for numbers

	s.Markdown = ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(fgBase.Hex()),
			},
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#4ADE80"), // Bright green for blockquotes
			},
			Indent:      uintPtr(1),
			IndentToken: stringPtr("│ "),
		},
		List: ansi.StyleList{
			LevelIndent: defaultListIndent,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       stringPtr(fgBase.Hex()),
				Bold:        boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: " ",
				Color:  stringPtr(primaryHex),
				Bold:   boolPtr(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "## ",
				Color:  stringPtr("#A855F7"), // Purple for H2
				Bold:   boolPtr(true),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "### ",
				Color:  stringPtr(primaryHex),
				Bold:   boolPtr(true),
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "#### ",
				Color:  stringPtr(markdownTertiaryHex),
				Bold:   boolPtr(true),
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "##### ",
				Color:  stringPtr(fgHalfMuted.Hex()),
				Bold:   boolPtr(false),
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "###### ",
				Color:  stringPtr(fgSubtle.Hex()),
				Bold:   boolPtr(false),
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: boolPtr(true),
		},
		Emph: ansi.StylePrimitive{
			Italic: boolPtr(true),
			Color:  stringPtr(markdownTertiaryHex),
		},
		Strong: ansi.StylePrimitive{
			Bold:  boolPtr(true),
			Color: stringPtr(primaryHex),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  stringPtr("#A855F7"), // Purple for horizontal rules
			Format: "\n---\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
			Color:       stringPtr("#4ADE80"), // Bright green for list items
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
			Color:       stringPtr("#F97316"), // Orange for enumerations
		},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{},
			Ticked:         "[✓] ",
			Unticked:       "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Color:     stringPtr("#A855F7"), // Purple for links
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: stringPtr("#F97316"), // Orange for link text
			Bold:  boolPtr(false),
		},
		Image: ansi.StylePrimitive{
			Color:     stringPtr(fgHalfMuted.Hex()),
			Underline: boolPtr(true),
		},
		ImageText: ansi.StylePrimitive{
			Color:  stringPtr(fgHalfMuted.Hex()),
			Format: "Image: {{.text}} →",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           stringPtr(syntaxGray),
				BackgroundColor: stringPtr("#181412"),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color:           stringPtr(fgBase.Hex()),
					BackgroundColor: stringPtr("#181412"),
				},
				Margin: uintPtr(1),
			},
			Chroma: &ansi.Chroma{
				Text: ansi.StylePrimitive{
					Color: stringPtr(fgBase.Hex()),
				},
				Error: ansi.StylePrimitive{
					Color:           stringPtr(syntaxGray),
					BackgroundColor: stringPtr("#181412"),
				},
				Comment: ansi.StylePrimitive{
					Color: stringPtr(syntaxGrayMuted),
				},
				CommentPreproc: ansi.StylePrimitive{
					Color: stringPtr(syntaxPurple),
				},
				Keyword: ansi.StylePrimitive{
					Color: stringPtr(syntaxPurple),
				},
				KeywordReserved: ansi.StylePrimitive{
					Color: stringPtr(syntaxPurple),
				},
				KeywordNamespace: ansi.StylePrimitive{
					Color: stringPtr(syntaxBlue),
				},
				KeywordType: ansi.StylePrimitive{
					Color: stringPtr(syntaxBlue),
					Bold:  boolPtr(true),
				},
				Operator: ansi.StylePrimitive{
					Color: stringPtr(syntaxGrayMuted),
				},
				Punctuation: ansi.StylePrimitive{
					Color: stringPtr(syntaxGrayMuted),
				},
				Name: ansi.StylePrimitive{
					Color: stringPtr(syntaxBlue),
				},
				NameBuiltin: ansi.StylePrimitive{
					Color: stringPtr(syntaxBlue),
				},
				NameTag: ansi.StylePrimitive{
					Color: stringPtr(syntaxBlue),
				},
				NameAttribute: ansi.StylePrimitive{
					Color: stringPtr(syntaxBlue),
				},
				NameClass: ansi.StylePrimitive{
					Color: stringPtr(syntaxBlue),
					Bold:  boolPtr(true),
				},
				NameConstant: ansi.StylePrimitive{
					Color: stringPtr(syntaxGreen),
					Bold:  boolPtr(true),
				},
				NameDecorator: ansi.StylePrimitive{
					Color: stringPtr(syntaxPurple),
				},
				NameFunction: ansi.StylePrimitive{
					Color: stringPtr(syntaxPink),
					Bold:  boolPtr(true),
				},
				NameException: ansi.StylePrimitive{
					Color: stringPtr(syntaxBlue),
				},
				NameOther: ansi.StylePrimitive{
					Color: stringPtr(syntaxBlue),
				},
				LiteralNumber: ansi.StylePrimitive{
					Color: stringPtr(syntaxLime),
					Bold:  boolPtr(true),
				},
				LiteralDate: ansi.StylePrimitive{
					Color: stringPtr(syntaxLime),
				},
				LiteralStringEscape: ansi.StylePrimitive{
					Color: stringPtr(syntaxOrange),
				},
				LiteralString: ansi.StylePrimitive{
					Color: stringPtr(syntaxOrange),
				},
				GenericDeleted: ansi.StylePrimitive{
					Color: stringPtr("#F38BA8"),
				},
				GenericEmph: ansi.StylePrimitive{
					Italic: boolPtr(true),
				},
				GenericInserted: ansi.StylePrimitive{
					Color: stringPtr("#A7D46F"),
				},
				GenericStrong: ansi.StylePrimitive{
					Bold: boolPtr(true),
				},
				GenericSubheading: ansi.StylePrimitive{
					Color: stringPtr(markdownSecondaryHex),
				},
				Background: ansi.StylePrimitive{
					BackgroundColor: stringPtr("#181412"),
				},
			},
		},
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{},
			},
		},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix: "\n ",
		},
	}

	// PlainMarkdown style - muted colors on warm background for thinking content.
	plainBg := stringPtr("#171311")
	plainFg := stringPtr("#fff3e9ff")
	s.PlainMarkdown = ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
			Indent:      uintPtr(1),
			IndentToken: stringPtr("│ "),
		},
		List: ansi.StyleList{
			LevelIndent: defaultListIndent,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix:     "\n",
				Bold:            boolPtr(false),
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Bold:            boolPtr(false),
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "## ",
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "### ",
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "#### ",
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "##### ",
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "###### ",
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut:      boolPtr(true),
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Emph: ansi.StylePrimitive{
			Italic:          boolPtr(true),
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Strong: ansi.StylePrimitive{
			Bold:            boolPtr(true),
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		HorizontalRule: ansi.StylePrimitive{
			Format:          "\n--------\n",
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Item: ansi.StylePrimitive{
			BlockPrefix:     "• ",
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix:     ". ",
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
			Ticked:   "[✓] ",
			Unticked: "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Underline:       boolPtr(true),
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		LinkText: ansi.StylePrimitive{
			Bold:            boolPtr(true),
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Image: ansi.StylePrimitive{
			Underline:       boolPtr(true),
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		ImageText: ansi.StylePrimitive{
			Format:          "Image: {{.text}} →",
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color:           plainFg,
					BackgroundColor: plainBg,
				},
				Margin: uintPtr(defaultMargin),
			},
		},
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color:           plainFg,
					BackgroundColor: plainBg,
				},
			},
		},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix:     "\n ",
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
	}

	s.Help = help.Styles{
		ShortKey:       base.Foreground(fgMuted),
		ShortDesc:      base.Foreground(fgSubtle),
		ShortSeparator: base.Foreground(border),
		Ellipsis:       base.Foreground(border),
		FullKey:        base.Foreground(fgMuted),
		FullDesc:       base.Foreground(fgSubtle),
		FullSeparator:  base.Foreground(border),
	}

	hunkBase := lipgloss.NewStyle().
		Foreground(fgSubtle).
		Background(bgBaseLighter)

	s.Diff = diffview.Style{
		HunkLine: diffview.HunkLineStyle{
			Base:  hunkBase,
			Minus: lipgloss.NewStyle().Foreground(red).Background(bgBaseLighter),
			Plus:  lipgloss.NewStyle().Foreground(greenLight).Background(bgBaseLighter),
		},
		DividerLine: diffview.LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(fgHalfMuted).
				Background(bgBaseLighter),
			Code: lipgloss.NewStyle().
				Foreground(fgHalfMuted).
				Background(bgBaseLighter),
		},
		MissingLine: diffview.LineStyle{
			LineNumber: lipgloss.NewStyle().
				Background(bgBaseLighter),
			Code: lipgloss.NewStyle().
				Background(bgBaseLighter),
		},
		EqualLine: diffview.LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(fgMuted).
				Background(bgBase),
			Code: lipgloss.NewStyle().
				Foreground(fgMuted).
				Background(bgBase),
		},
		InsertLine: diffview.LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(greenLight).
				Background(lipgloss.Color("#16231B")),
			Symbol: lipgloss.NewStyle().
				Foreground(greenLight).
				Background(lipgloss.Color("#1D2D22")),
			Code: lipgloss.NewStyle().
				Background(lipgloss.Color("#1D2D22")),
		},
		DeleteLine: diffview.LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(red).
				Background(lipgloss.Color("#24161D")),
			Symbol: lipgloss.NewStyle().
				Foreground(red).
				Background(lipgloss.Color("#2C1C24")),
			Code: lipgloss.NewStyle().
				Background(lipgloss.Color("#2C1C24")),
		},
	}

	s.FilePicker = filepicker.Styles{
		DisabledCursor:   base.Foreground(fgMuted),
		Cursor:           base.Foreground(fgBase),
		Symlink:          base.Foreground(fgSubtle),
		Directory:        base.Foreground(blueLight),
		File:             base.Foreground(fgBase),
		DisabledFile:     base.Foreground(fgMuted),
		DisabledSelected: base.Background(bgBaseLighter).Foreground(fgMuted),
		Permission:       base.Foreground(fgMuted),
		Selected:         base.Background(bgOverlay).Foreground(primary).Bold(true),
		FileSize:         base.Foreground(fgMuted),
		EmptyDirectory:   base.Foreground(fgMuted).PaddingLeft(2).SetString("Empty directory"),
	}

	// borders
	s.FocusedMessageBorder = lipgloss.Border{Left: BorderThick}

	// text presets
	s.Base = lipgloss.NewStyle().Foreground(fgBase)
	s.Muted = lipgloss.NewStyle().Foreground(fgMuted)
	s.HalfMuted = lipgloss.NewStyle().Foreground(fgHalfMuted)
	s.Subtle = lipgloss.NewStyle().Foreground(fgSubtle)

	s.WindowTooSmall = s.Muted

	// tag presets
	s.TagBase = lipgloss.NewStyle().Padding(0, 1).Foreground(white)
	s.TagError = s.TagBase.Background(redDark)
	s.TagInfo = s.TagBase.Background(secondary)

	// Compact header styles
	s.Header.Charm = base.Foreground(secondary)
	s.Header.Diagonals = base.Foreground(primary)
	s.Header.Percentage = s.Muted
	s.Header.Keystroke = s.Muted
	s.Header.KeystrokeTip = s.Subtle
	s.Header.WorkingDir = s.Muted
	s.Header.Separator = s.Subtle

	s.CompactDetails.Title = s.Base.Foreground(secondary)
	s.CompactDetails.View = s.Base.Padding(0, 1, 1, 1).Background(bgOverlay).Border(lipgloss.RoundedBorder()).BorderForeground(border)
	s.CompactDetails.Version = s.Muted

	// panels
	s.PanelMuted = s.Muted.Background(bgBase)
	s.PanelBase = lipgloss.NewStyle().Background(bgBase)
	s.PanelPadded = lipgloss.NewStyle().Padding(1, 2)

	// code line number
	s.LineNumber = lipgloss.NewStyle().Foreground(fgMuted).Background(bgBaseLighter).PaddingRight(1).PaddingLeft(1)

	// Tool calls
	s.ToolCallPending = lipgloss.NewStyle().Foreground(primary).SetString(ToolPending)
	s.ToolCallError = lipgloss.NewStyle().Foreground(red).SetString(ToolError)
	s.ToolCallSuccess = lipgloss.NewStyle().Foreground(yellow).SetString(ToolSuccess)
	// Cancelled uses muted tone but same glyph as pending
	s.ToolCallCancelled = s.Muted.SetString(ToolPending)
	s.EarlyStateMessage = s.Subtle.PaddingLeft(2)

	// Tool rendering styles
	// Icon: orange for pending operations (●)
	s.Tool.IconPending = base.Foreground(primary).SetString(ToolPending)
	// Success: warm amber for checkmarks (✓)
	s.Tool.IconSuccess = base.Foreground(yellow).SetString(ToolSuccess)
	s.Tool.IconError = base.Foreground(red).SetString(ToolError)
	s.Tool.IconCancelled = s.Muted.SetString(ToolPending)

	// Tool names: warm accent emphasis
	s.Tool.NameNormal = base.Foreground(secondary)
	s.Tool.NameNested = base.Foreground(fgHalfMuted)

	s.Tool.ParamMain = s.Muted
	s.Tool.ParamKey = s.Subtle

	// Content rendering - prepared styles that accept width parameter
	s.Tool.ContentLine = s.Base.Foreground(fgBase).Background(bgOverlay).Padding(0, 1)
	s.Tool.ContentTruncation = s.Muted.Background(bgOverlay).Padding(0, 1)
	s.Tool.ContentCodeLine = s.Base.Background(bgOverlay).Padding(0, 1)
	s.Tool.ContentCodeTruncation = s.Muted.Background(bgOverlay).Padding(0, 1)
	s.Tool.ContentCodeBg = bgOverlay
	s.Tool.Body = base.PaddingLeft(2)
	s.Tool.FileBlock = base.Background(bgOverlay).Padding(0, 1)

	// Deprecated - kept for backward compatibility
	s.Tool.ContentBg = s.Muted.Background(bgOverlay)
	s.Tool.ContentText = s.Muted
	s.Tool.ContentLineNumber = base.Foreground(fgHalfMuted).Background(bgSubtle).PaddingRight(1).PaddingLeft(1)

	s.Tool.StateWaiting = base.Foreground(fgSubtle)
	s.Tool.StateCancelled = base.Foreground(fgSubtle)

	s.Tool.ErrorTag = base.Foreground(red).Bold(true)
	s.Tool.ErrorMessage = base.Foreground(fgBase)

	// Diff and multi-edit styles
	s.Tool.DiffTruncation = s.Muted.Background(bgBaseLighter).PaddingLeft(2)
	s.Tool.NoteTag = base.Padding(0, 1).Background(bgBaseLighter).Foreground(lipgloss.Color("#D4D0CB")).Bold(true)
	s.Tool.NoteMessage = base.Foreground(fgHalfMuted)

	// Job header styles - warm accents
	s.Tool.JobIconPending = base.Foreground(primary)
	s.Tool.JobIconError = base.Foreground(red)
	s.Tool.JobIconSuccess = base.Foreground(greenLight)
	s.Tool.JobToolName = base.Foreground(secondary)
	s.Tool.JobAction = base.Foreground(fgHalfMuted)
	s.Tool.JobPID = s.Muted
	s.Tool.JobDescription = s.Subtle
	s.Tool.BashLabel = base.Foreground(lipgloss.Color("#D4D0CB")).Background(bgSubtle).Padding(0, 1).Bold(true)
	s.Tool.BashCommand = base.Foreground(fgBase).Background(bgSubtle).Padding(0, 1)
	s.Tool.BashOutputLabel = base.Foreground(greenLight).Bold(true)

	// Agent task styles - warm orange accent
	s.Tool.AgentTaskTag = base.Bold(true).Padding(0, 1).MarginLeft(2).Background(bgOverlay).Foreground(lipgloss.Color("#E79A5B"))
	s.Tool.AgentPrompt = s.Muted

	// Agentic fetch styles - orange accent
	s.Tool.AgenticFetchPromptTag = base.Bold(true).Padding(0, 1).MarginLeft(2).Background(bgOverlay).Foreground(lipgloss.Color("#E28AB0"))

	// Todo styles
	s.Tool.TodoRatio = base.Foreground(secondary)
	s.Tool.TodoCompletedIcon = base.Foreground(greenLight)
	s.Tool.TodoInProgressIcon = base.Foreground(greenLight)
	s.Tool.TodoPendingIcon = base.Foreground(fgMuted)

	// MCP styles
	s.Tool.MCPName = base.Foreground(secondary)
	s.Tool.MCPToolName = base.Foreground(fgHalfMuted)
	s.Tool.MCPArrow = base.Foreground(secondary).SetString(ArrowRightIcon)

	s.Tool.ListRoot = base.Foreground(fgHalfMuted)
	s.Tool.ListDirectory = base.Foreground(lipgloss.Color("#D4D0CB")).Bold(true)
	s.Tool.ListFile = base.Foreground(fgBase)
	s.Tool.ListMeta = base.Foreground(fgMuted)
	s.Tool.ListHint = base.Foreground(warning)
	s.Tool.GrepFile = base.Foreground(lipgloss.Color("#D4D0CB")).Bold(true)
	s.Tool.GrepLine = base.Foreground(warning)
	s.Tool.GrepMatch = base.Foreground(primary).Bold(true)
	s.Tool.GrepContext = base.Foreground(fgMuted)

	// Loading indicators for images, skills
	s.Tool.ResourceLoadedText = base.Foreground(lipgloss.Color("#E79A5B"))
	s.Tool.ResourceLoadedIndicator = base.Foreground(greenDark)
	s.Tool.ResourceName = base
	s.Tool.MediaType = base
	s.Tool.ResourceSize = base.Foreground(fgMuted)
	s.Tool.SkillTag = base.Bold(true).Padding(0, 1).Background(bgSubtle).Foreground(lipgloss.Color("#B78AE8"))

	// Buttons
	s.ButtonFocus = lipgloss.NewStyle().Foreground(white).Background(primary)
	s.ButtonBlur = s.Base.Background(bgOverlay)

	// Borders
	s.BorderFocus = lipgloss.NewStyle().BorderForeground(borderFocus).Border(lipgloss.RoundedBorder()).Background(bgOverlay).Padding(1, 2)

	// Editor - use warm amber tones instead of green
	s.EditorPromptNormalFocused = lipgloss.NewStyle().Foreground(greenDark).SetString("::: ")
	s.EditorPromptNormalBlurred = s.EditorPromptNormalFocused.Foreground(fgMuted)
	yoloIconBg := greenDark
	statusWarn := warning
	statusOk := greenLight
	s.EditorPromptStatusIconFocused = lipgloss.NewStyle().Foreground(white).Background(yoloIconBg).Bold(true)
	s.EditorPromptStatusIconBlurred = s.EditorPromptStatusIconFocused
	s.EditorPromptStatusIconWarnFocused = lipgloss.NewStyle().Foreground(bgBase).Background(statusWarn).Bold(true)
	s.EditorPromptStatusIconWarnBlurred = s.EditorPromptStatusIconWarnFocused
	s.EditorPromptStatusDotsWarn = lipgloss.NewStyle().Foreground(statusWarn)
	s.EditorPromptStatusDotsWarnMuted = lipgloss.NewStyle().Foreground(fgHalfMuted)
	s.EditorPromptStatusDotsOk = lipgloss.NewStyle().Foreground(statusOk)
	s.EditorPromptStatusDotsOkMuted = lipgloss.NewStyle().Foreground(fgHalfMuted)
	s.EditorPromptYoloIconFocused = lipgloss.NewStyle().MarginRight(1).Foreground(white).Background(greenDark).Bold(true).SetString(" ! ")
	s.EditorPromptYoloIconBlurred = s.EditorPromptYoloIconFocused.Foreground(charmtone.Pepper).Background(lipgloss.Color("#1f3b27"))
	s.EditorPromptYoloDotsFocused = lipgloss.NewStyle().MarginRight(1).Foreground(greenLight).SetString(":::")
	s.EditorPromptYoloDotsBlurred = s.EditorPromptYoloDotsFocused.Foreground(lipgloss.Color("#4b7a58"))

	s.RadioOn = s.HalfMuted.SetString(RadioOn)
	s.RadioOff = s.HalfMuted.SetString(RadioOff)

	// Logo colors
	s.LogoFieldColor = primary
	s.LogoTitleColorA = secondary
	s.LogoTitleColorB = primary
	s.LogoCharmColor = secondary
	s.LogoVersionColor = primary

	// Section - use primary orange for stronger headers
	s.Section.Title = s.Base.Foreground(primary)
	s.Section.Line = s.Base.Foreground(charmtone.Charcoal)

	// Initialize - use warm amber for accent
	s.Initialize.Header = s.Base
	s.Initialize.Content = s.Muted
	s.Initialize.Accent = s.Base.Foreground(primary)

	// LSP and MCP status.
	s.ResourceGroupTitle = lipgloss.NewStyle().Foreground(fgHalfMuted)
	mcpOk := greenLight
	mcpWarn := warning
	mcpError := red
	s.ResourceOfflineIcon = lipgloss.NewStyle().Foreground(mcpError).SetString("●")
	s.ResourceBusyIcon = lipgloss.NewStyle().Foreground(mcpWarn).SetString("●")
	s.ResourceErrorIcon = lipgloss.NewStyle().Foreground(mcpError).SetString("●")
	s.ResourceOnlineIcon = lipgloss.NewStyle().Foreground(mcpOk).SetString("●")
	s.ResourceName = lipgloss.NewStyle().Foreground(fgBase)
	s.ResourceStatus = lipgloss.NewStyle().Foreground(fgHalfMuted)
	s.ResourceAdditionalText = lipgloss.NewStyle().Foreground(fgHalfMuted)

	// LSP
	s.LSP.ErrorDiagnostic = s.Base.Foreground(redDark)
	s.LSP.WarningDiagnostic = s.Base.Foreground(warning)
	s.LSP.HintDiagnostic = s.Base.Foreground(fgHalfMuted)
	s.LSP.InfoDiagnostic = s.Base.Foreground(info)

	// Files - use warm amber for additions
	s.Files.Path = s.Muted
	s.Files.Additions = s.Base.Foreground(greenLight)
	s.Files.Deletions = s.Base.Foreground(red)

	// Chat
	messageFocussedBorder := lipgloss.Border{
		Left: "▌",
	}

	s.Chat.Message.NoContent = lipgloss.NewStyle().Foreground(fgBase)
	s.Chat.Message.UserBlurred = s.Chat.Message.NoContent.PaddingLeft(1).BorderLeft(true).
		BorderForeground(tertiary).BorderStyle(normalBorder)
	s.Chat.Message.UserFocused = s.Chat.Message.NoContent.PaddingLeft(1).BorderLeft(true).
		BorderForeground(primary).BorderStyle(messageFocussedBorder)
	s.Chat.Message.AssistantBlurred = s.Chat.Message.NoContent.PaddingLeft(2)
	s.Chat.Message.AssistantFocused = s.Chat.Message.NoContent.PaddingLeft(1).BorderLeft(true).
		BorderForeground(secondary).BorderStyle(messageFocussedBorder)
	s.Chat.Message.Thinking = lipgloss.NewStyle().MaxHeight(10)
	s.Chat.Message.ErrorTag = lipgloss.NewStyle().
		Foreground(red).
		Bold(true)
	s.Chat.Message.ErrorTitle = lipgloss.NewStyle().
		Foreground(fgBase)
	s.Chat.Message.ErrorDetails = lipgloss.NewStyle().
		Foreground(fgHalfMuted).
		PaddingLeft(2)

	// Message item styles
	s.Chat.Message.ToolCallFocused = s.Muted.PaddingLeft(1).
		BorderStyle(messageFocussedBorder).
		BorderLeft(true).
		BorderForeground(secondary)
	s.Chat.Message.ToolCallBlurred = s.Muted.PaddingLeft(2)
	// No padding or border for compact tool calls within messages
	s.Chat.Message.ToolCallCompact = s.Muted
	s.Chat.Message.SectionHeader = s.Base.PaddingLeft(2)
	s.Chat.Message.AssistantInfoIcon = s.Subtle
	s.Chat.Message.AssistantInfoModel = s.Muted
	s.Chat.Message.AssistantInfoProvider = s.Subtle
	s.Chat.Message.AssistantInfoDuration = s.Subtle

	s.Chat.Message.ThinkingBox = s.Base.Background(thinkingBg).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(thinkingBorder).
		MarginBottom(1)

	// Thinking section styles
	s.Chat.Message.ThinkingTruncationHint = s.HalfMuted
	s.Chat.Message.ThinkingFooterTitle = s.HalfMuted
	s.Chat.Message.ThinkingFooterDuration = s.Muted

	// Text selection.
	s.TextSelection = lipgloss.NewStyle().Foreground(charmtone.Salt).Background(secondary)

	// Dialog styles
	s.Dialog.Title = base.Padding(0, 1).Foreground(primary)
	s.Dialog.TitleText = base.Foreground(primary)
	s.Dialog.TitleError = base.Foreground(red)
	s.Dialog.TitleAccent = base.Foreground(secondary).Bold(true)
	dialogSurface := bgOverlay
	s.Dialog.View = base.Border(lipgloss.RoundedBorder()).BorderForeground(secondary).Background(dialogSurface)
	s.Dialog.PrimaryText = base.Padding(0, 1).Foreground(primary)
	s.Dialog.SecondaryText = base.Padding(0, 1).Foreground(fgSubtle)
	s.Dialog.HelpView = base.Padding(0, 1).AlignHorizontal(lipgloss.Left)
	s.Dialog.Help.ShortKey = base.Foreground(fgMuted)
	s.Dialog.Help.ShortDesc = base.Foreground(fgSubtle)
	s.Dialog.Help.ShortSeparator = base.Foreground(secondary)
	s.Dialog.Help.Ellipsis = base.Foreground(border)
	s.Dialog.Help.FullKey = base.Foreground(fgMuted)
	s.Dialog.Help.FullDesc = base.Foreground(fgSubtle)
	s.Dialog.Help.FullSeparator = base.Foreground(secondary)
	s.Dialog.NormalItem = base.Padding(0, 1).Foreground(fgBase)
	s.Dialog.SelectedItem = base.Padding(0, 1).Background(secondary).Foreground(bgBase)
	s.Dialog.InputPrompt = base.Margin(1, 1)

	s.Dialog.List = base.Margin(0, 0, 1, 0)
	s.Dialog.ContentPanel = base.Background(dialogSurface).Foreground(fgBase).Padding(1, 2)
	s.Dialog.Spinner = base.Foreground(secondary)
	s.Dialog.ScrollbarThumb = base.Foreground(secondary)
	s.Dialog.ScrollbarTrack = base.Foreground(border)
	s.Dialog.View = base.Border(lipgloss.RoundedBorder()).BorderForeground(secondary).Background(dialogSurface)

	s.Dialog.ImagePreview = lipgloss.NewStyle().Padding(0, 1).Foreground(fgSubtle)

	s.Dialog.Arguments.Content = base.Padding(1)
	s.Dialog.Arguments.Description = base.MarginBottom(1).MaxHeight(3)
	s.Dialog.Arguments.InputLabelBlurred = base.Foreground(fgMuted)
	s.Dialog.Arguments.InputLabelFocused = base.Bold(true)
	s.Dialog.Arguments.InputRequiredMarkBlurred = base.Foreground(fgMuted).SetString("*")
	s.Dialog.Arguments.InputRequiredMarkFocused = base.Foreground(primary).Bold(true).SetString("*")

	s.Dialog.Sessions.DeletingTitle = s.Dialog.Title.Foreground(red)
	s.Dialog.Sessions.DeletingView = s.Dialog.View.BorderForeground(red)
	s.Dialog.Sessions.DeletingMessage = s.Base.Padding(1)
	s.Dialog.Sessions.DeletingTitleGradientFromColor = red
	s.Dialog.Sessions.DeletingTitleGradientToColor = s.Primary
	s.Dialog.Sessions.DeletingItemBlurred = s.Dialog.NormalItem.Foreground(fgSubtle)
	s.Dialog.Sessions.DeletingItemFocused = s.Dialog.SelectedItem.Background(red).Foreground(white)

	s.Dialog.Sessions.RenamingingTitle = s.Dialog.Title.Foreground(warning)
	s.Dialog.Sessions.RenamingView = s.Dialog.View.BorderForeground(warning)
	s.Dialog.Sessions.RenamingingMessage = s.Base.Padding(1)
	s.Dialog.Sessions.RenamingTitleGradientFromColor = warning
	s.Dialog.Sessions.RenamingTitleGradientToColor = primary
	s.Dialog.Sessions.RenamingItemBlurred = s.Dialog.NormalItem.Foreground(fgSubtle)
	s.Dialog.Sessions.RenamingingItemFocused = s.Dialog.SelectedItem.UnsetBackground().UnsetForeground()
	s.Dialog.Sessions.RenamingPlaceholder = base.Foreground(fgHalfMuted)

	s.Status.Help = lipgloss.NewStyle().Padding(0, 1)
	s.Status.SuccessIndicator = lipgloss.NewStyle().Foreground(white).Background(toastSuccessBorder).Padding(0, 1).Bold(true).SetString("OK")
	s.Status.InfoIndicator = lipgloss.NewStyle().Foreground(white).Background(toastInfoBorder).Padding(0, 1).Bold(true).SetString("INFO")
	s.Status.UpdateIndicator = lipgloss.NewStyle().Foreground(white).Background(toastUpdateBorder).Padding(0, 1).Bold(true).SetString("UPDATE")
	s.Status.WarnIndicator = lipgloss.NewStyle().Foreground(white).Background(toastWarnBorder).Padding(0, 1).Bold(true).SetString("WARN")
	s.Status.ErrorIndicator = lipgloss.NewStyle().Foreground(white).Background(toastErrorBorder).Padding(0, 1).Bold(true).SetString("ERROR")
	s.Status.SuccessMessage = base.Foreground(white).Background(toastSuccessBg).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(toastSuccessBorder).Padding(0, 2)
	s.Status.InfoMessage = base.Foreground(white).Background(toastInfoBg).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(toastInfoBorder).Padding(0, 2)
	s.Status.UpdateMessage = base.Foreground(white).Background(toastUpdateBg).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(toastUpdateBorder).Padding(0, 2)
	s.Status.WarnMessage = base.Foreground(white).Background(toastWarnBg).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(toastWarnBorder).Padding(0, 2)
	s.Status.ErrorMessage = base.Foreground(white).Background(toastErrorBg).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(toastErrorBorder).Padding(0, 2)

	// Completions styles
	s.Completions.Normal = base.Background(bgBaseLighter).Foreground(fgBase)
	s.Completions.Focused = base.Background(primary).Foreground(bgBase)
	s.Completions.Match = base.Underline(true).Foreground(primary)

	// Attachments styles - minimal, clean design
	attachmentIconStyle := base.Foreground(bgBase).Background(yellow).Padding(0, 1)
	s.Attachments.Image = attachmentIconStyle.SetString(ImageIcon)
	s.Attachments.Text = attachmentIconStyle.SetString(TextIcon)
	s.Attachments.Normal = base.Padding(0, 1).MarginRight(1).Background(bgSubtle).Foreground(fgBase)
	s.Attachments.Deleting = base.Padding(0, 1).Bold(true).Background(red).Foreground(white)
	// Paste block: subtle surface contrast
	s.Attachments.PasteBlock = base.Padding(0, 1).MarginRight(1).Background(bgSubtle).Foreground(fgBase)
	s.Attachments.PasteSelected = base.Padding(0, 1).MarginRight(1).Background(bgOverlay).Foreground(fgBase).Bold(true)
	// Subtle palette for paste blocks
	s.Attachments.PastePalette = []color.Color{
		bgSubtle,
		bgSubtle,
		bgSubtle,
		bgSubtle,
		bgSubtle,
	}
	// Selected palette (slightly lighter)
	s.Attachments.PasteSelectedPalette = []color.Color{
		bgOverlay,
		bgOverlay,
		bgOverlay,
		bgOverlay,
		bgOverlay,
	}

	// Pills styles - use warm amber for todo spinner
	s.Pills.Base = base.Padding(0, 1)
	s.Pills.Focused = base.Padding(0, 1).Background(bgOverlay).Foreground(fgBase)
	s.Pills.Blurred = base.Padding(0, 1).Background(bgSubtle).Foreground(fgBase)
	s.Pills.QueueItemPrefix = s.Muted.SetString("  •")
	s.Pills.HelpKey = s.Muted
	s.Pills.HelpText = s.Subtle
	s.Pills.Area = base
	s.Pills.TodoSpinner = base.Foreground(greenLight)

	return s
}

// Helper functions for style pointers
func boolPtr(b bool) *bool       { return &b }
func stringPtr(s string) *string { return &s }
func uintPtr(u uint) *uint       { return &u }
func chromaStyle(style ansi.StylePrimitive) string {
	var s strings.Builder

	if style.Color != nil {
		s.WriteString(*style.Color)
	}
	if style.BackgroundColor != nil {
		if s.Len() > 0 {
			s.WriteString(" ")
		}
		s.WriteString("bg:")
		s.WriteString(*style.BackgroundColor)
	}
	if style.Italic != nil && *style.Italic {
		if s.Len() > 0 {
			s.WriteString(" ")
		}
		s.WriteString("italic")
	}
	if style.Bold != nil && *style.Bold {
		if s.Len() > 0 {
			s.WriteString(" ")
		}
		s.WriteString("bold")
	}
	if style.Underline != nil && *style.Underline {
		if s.Len() > 0 {
			s.WriteString(" ")
		}
		s.WriteString("underline")
	}

	return s.String()
}
