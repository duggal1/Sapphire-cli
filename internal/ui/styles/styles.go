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
	ToolSuccess string = "●"
	ToolError   string = "■"

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

	ScrollbarThumb string = " "
	ScrollbarTrack string = " "

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
		NameNormal        lipgloss.Style // Normal tool name
		NameNested        lipgloss.Style // Nested tool name
		NameSuccess       lipgloss.Style // Success tool name
		NameSuccessNested lipgloss.Style // Success nested tool name

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
		TodoFailedIcon     lipgloss.Style // Failed todo icon
		TodoCanceledIcon   lipgloss.Style // Canceled todo icon

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

// Legacy alternate-accent colors. The toggle remains for compatibility, but
// the alternate palette stays within the same purple/pink family.
var (
	yellowPrimaryHex   = "#A34DFF"
	yellowSecondaryHex = "#EA8EED"
	yellowTertiaryHex  = "#C58BFF"
	yellowPrimary      = lipgloss.Color(yellowPrimaryHex)
	yellowSecondary    = lipgloss.Color(yellowSecondaryHex)
	yellowTertiary     = lipgloss.Color(yellowTertiaryHex)
)

// DefaultStyles returns the default styles for the UI.
func DefaultStyles(yellowMode bool) Styles {
	var (
		// Semantic Theme Colors - purple-led palette with a calmer pink accent.
		primaryHex   = "#A34DFF"
		primary      = lipgloss.Color(primaryHex)
		secondaryHex = "#EA8EED"
		secondary    = lipgloss.Color(secondaryHex)
		highlightHex = "#C7B8FF"
		highlight    = lipgloss.Color(highlightHex)
		tertiaryHex  = "#C58BFF"
		tertiary     = lipgloss.Color(tertiaryHex)
		logoPinkHex  = "#FF8FE7"
		logoPink     = lipgloss.Color(logoPinkHex)
		// Markdown accent pair (Purple + Pink)
		markdownSecondaryHex = secondaryHex
		markdownTertiaryHex  = tertiaryHex

		// Backgrounds - anchored to the agentic terminal base while keeping
		// overlays lighter than the canvas.
		bgBaseHex        = "#15141B"
		bgBaseLighterHex = "#1B1924"
		bgSubtleHex      = "#191722"
		bgOverlayHex     = "#211E2E"
		bgBase           = lipgloss.Color(bgBaseHex)
		bgBaseLighter    = lipgloss.Color(bgBaseLighterHex)
		bgSubtle         = lipgloss.Color(bgSubtleHex)
		bgOverlay        = lipgloss.Color(bgOverlayHex)

		thinkingBorder = lipgloss.Color("#44336A")

		// Foregrounds
		fgBase      = charmtone.Ash
		fgMuted     = charmtone.Squid
		fgHalfMuted = charmtone.Smoke
		fgSubtle    = charmtone.Oyster

		// Borders
		border      = lipgloss.Color("#33294A")
		borderFocus = lipgloss.Color("#7B3FF2")

		// Status palette
		error   = lipgloss.Color("#FF7AA8")
		warning = secondary
		info    = tertiary

		// Toast backgrounds (status bar)
		toastSuccessBg     = lipgloss.Color("#00aa44ff")
		toastSuccessBorder = lipgloss.Color("#166534")
		toastInfoBg        = lipgloss.Color("#2B2244")
		toastInfoBorder    = lipgloss.Color(primaryHex)
		toastUpdateBg      = lipgloss.Color("#2B2244")
		toastUpdateBorder  = lipgloss.Color(primaryHex)
		toastWarnBg        = lipgloss.Color("#173945")
		toastWarnBorder    = lipgloss.Color(tertiaryHex)
		toastErrorBg       = lipgloss.Color("#991B1B")
		toastErrorBorder   = lipgloss.Color("#B91C1C")

		// Toast overlay colors
		toastSuccessColor = charmtone.Guac
		toastWarnColor    = warning
		toastErrorColor   = error
		toastInfoColor    = primary
		toastTextColor    = charmtone.Butter

		// Colors
		white = charmtone.Butter

		// Support accent
		blueLight = tertiary
		blue      = tertiary
		blueDark  = tertiary

		yellowHex = secondaryHex
		yellow    = lipgloss.Color(yellowHex)

		// Bright green for todo ticks and success states
		greenLight = lipgloss.Color("#5AF2B3")
		green      = charmtone.Guac
		greenDark  = charmtone.Guac

		red     = lipgloss.Color("#FF7AA8")
		redDark = lipgloss.Color("#E25386")
	)

	if yellowMode {
		primary = yellowPrimary
		secondary = yellowSecondary
		tertiary = yellowTertiary
		primaryHex = yellowPrimaryHex
		secondaryHex = yellowSecondaryHex
		tertiaryHex = yellowTertiaryHex
	}

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
			Prompt:      base.Foreground(primary),
			Suggestion:  base.Foreground(fgSubtle),
		},
		Blurred: textinput.StyleState{
			Text:        base.Foreground(fgMuted),
			Placeholder: base.Foreground(fgSubtle),
			Prompt:      base.Foreground(fgMuted),
			Suggestion:  base.Foreground(fgSubtle),
		},
		Cursor: textinput.CursorStyle{
			Color: primary,
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
			Prompt:           base.Foreground(primary),
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
			Color: primary,
			Shape: tea.CursorBlock,
			Blink: true,
		},
	}

	syntaxOffWhite := "#E6E6E6"  // Plain text / foreground
	syntaxPurple := "#A34DFF"    // Keywords / Operators
	syntaxPink := "#EA8EED"      // Functions / Methods
	syntaxOrange := "#FF944D"    // Strings
	syntaxLavender := "#D4B3FF"  // Types / Classes
	syntaxLightBlue := "#7FD4FF" // Variables / Properties
	syntaxLightRose := "#FFB4C2" // Errors
	syntaxNeonCyan := "#00FFD9"  // Comments

	s.Markdown = ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(fgBase.Hex()),
			},
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(tertiaryHex),
			},
			Indent:      uintPtr(1),
			IndentToken: stringPtr("  "),
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
				Color:  stringPtr(primaryHex),
				Bold:   boolPtr(true),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "### ",
				Color:  stringPtr(markdownSecondaryHex),
				Bold:   boolPtr(true),
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "#### ",
				Color:  stringPtr(primaryHex),
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
			Color:  stringPtr(primaryHex),
			Format: "\n---\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
			Color:       stringPtr(primaryHex),
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
			Color:       stringPtr(primaryHex),
		},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{},
			Ticked:         "[✓] ",
			Unticked:       "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Color:     stringPtr(primaryHex),
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: stringPtr(markdownSecondaryHex),
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
				Color:           stringPtr(syntaxOffWhite),
				BackgroundColor: stringPtr("#1A1624"),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color:           stringPtr(syntaxOffWhite),
					BackgroundColor: stringPtr("#1A1624"),
				},
				Margin: uintPtr(1),
			},
			Chroma: &ansi.Chroma{
				Text: ansi.StylePrimitive{
					Color: stringPtr(syntaxOffWhite),
				},
				Error: ansi.StylePrimitive{
					Color:           stringPtr(syntaxLightRose),
					BackgroundColor: stringPtr("#1A1624"),
				},
				Comment: ansi.StylePrimitive{
					Color: stringPtr(syntaxNeonCyan),
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
					Color: stringPtr(syntaxLavender),
				},
				KeywordType: ansi.StylePrimitive{
					Color: stringPtr(syntaxLavender),
					Bold:  boolPtr(true),
				},
				Operator: ansi.StylePrimitive{
					Color: stringPtr(syntaxPurple),
				},
				Punctuation: ansi.StylePrimitive{
					Color: stringPtr(syntaxOffWhite),
				},
				Name: ansi.StylePrimitive{
					Color: stringPtr(syntaxLightBlue),
				},
				NameBuiltin: ansi.StylePrimitive{
					Color: stringPtr(syntaxLightBlue),
				},
				NameTag: ansi.StylePrimitive{
					Color: stringPtr(syntaxLightBlue),
				},
				NameAttribute: ansi.StylePrimitive{
					Color: stringPtr(syntaxLightBlue),
				},
				NameClass: ansi.StylePrimitive{
					Color: stringPtr(syntaxLavender),
					Bold:  boolPtr(true),
				},
				NameConstant: ansi.StylePrimitive{
					Color: stringPtr(syntaxLightBlue),
				},
				NameDecorator: ansi.StylePrimitive{
					Color: stringPtr(syntaxPink),
				},
				NameFunction: ansi.StylePrimitive{
					Color: stringPtr(syntaxPink),
					Bold:  boolPtr(true),
				},
				NameException: ansi.StylePrimitive{
					Color: stringPtr(syntaxLavender),
				},
				NameOther: ansi.StylePrimitive{
					Color: stringPtr(syntaxLightBlue),
				},
				LiteralNumber: ansi.StylePrimitive{
					Color: stringPtr(syntaxOrange),
				},
				LiteralDate: ansi.StylePrimitive{
					Color: stringPtr(syntaxOrange),
				},
				LiteralStringEscape: ansi.StylePrimitive{
					Color: stringPtr(syntaxOrange),
				},
				LiteralString: ansi.StylePrimitive{
					Color: stringPtr(syntaxOrange),
				},
				GenericDeleted: ansi.StylePrimitive{
					Color: stringPtr(syntaxLightRose),
				},
				GenericEmph: ansi.StylePrimitive{
					Italic: boolPtr(true),
				},
				GenericInserted: ansi.StylePrimitive{
					Color: stringPtr(syntaxLightBlue),
				},
				GenericStrong: ansi.StylePrimitive{
					Bold: boolPtr(true),
				},
				GenericSubheading: ansi.StylePrimitive{
					Color: stringPtr(markdownSecondaryHex),
				},
				Background: ansi.StylePrimitive{
					BackgroundColor: stringPtr("#1A1624"),
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

	// PlainMarkdown style - multi-color for thinking content.
	plainBg := (*string)(nil)
	plainFg := stringPtr("#F4EEFF")
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
			IndentToken: stringPtr("  "),
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
				Bold:            boolPtr(true),
				Color:           stringPtr(primaryHex),
				BackgroundColor: plainBg,
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "## ",
				Color:           stringPtr(primaryHex),
				BackgroundColor: plainBg,
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "### ",
				Color:           stringPtr(tertiaryHex),
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
			Minus: lipgloss.NewStyle().Foreground(red).Background(lipgloss.Color("#241521")).Bold(true),
			Plus:  lipgloss.NewStyle().Foreground(greenLight).Background(lipgloss.Color("#11251F")).Bold(true),
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
				Background(lipgloss.Color("#10261F")),
			Symbol: lipgloss.NewStyle().
				Foreground(greenLight).
				Background(lipgloss.Color("#143128")),
			Code: lipgloss.NewStyle().
				Background(lipgloss.Color("#143128")),
		},
		DeleteLine: diffview.LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(red).
				Background(lipgloss.Color("#311623")),
			Symbol: lipgloss.NewStyle().
				Foreground(red).
				Background(lipgloss.Color("#3B1B2A")),
			Code: lipgloss.NewStyle().
				Background(lipgloss.Color("#3B1B2A")),
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
	s.FocusedMessageBorder = lipgloss.Border{}

	// text presets
	s.Base = lipgloss.NewStyle().Foreground(fgBase)
	s.Muted = lipgloss.NewStyle().Foreground(fgMuted)
	s.HalfMuted = lipgloss.NewStyle().Foreground(fgHalfMuted)
	s.Subtle = lipgloss.NewStyle().Foreground(fgSubtle)

	s.WindowTooSmall = s.Muted

	// tag presets
	s.TagBase = lipgloss.NewStyle().Padding(0, 1).Foreground(white)
	s.TagError = s.TagBase.Background(redDark)
	s.TagInfo = s.TagBase.Background(primary)

	// Compact header styles
	s.Header.Charm = base.Foreground(primary)
	s.Header.Diagonals = base.Foreground(primary)
	s.Header.Percentage = s.Muted
	s.Header.Keystroke = s.Muted
	s.Header.KeystrokeTip = s.Subtle
	s.Header.WorkingDir = s.Muted
	s.Header.Separator = s.Subtle

	s.CompactDetails.Title = s.Base.Foreground(primary).Bold(true)
	s.CompactDetails.View = s.Base.Padding(0, 1).Background(bgBase).Border(lipgloss.NormalBorder()).BorderForeground(thinkingBorder)
	s.CompactDetails.Version = s.Muted

	// panels
	s.PanelMuted = s.Muted.Background(bgBase)
	s.PanelBase = lipgloss.NewStyle().Background(bgBase)
	s.PanelPadded = lipgloss.NewStyle().Padding(1, 2)

	// code line number
	s.LineNumber = lipgloss.NewStyle().Foreground(fgMuted).Background(bgBaseLighter).PaddingRight(1).PaddingLeft(1)

	// Tool calls
	s.ToolCallPending = lipgloss.NewStyle().Foreground(fgHalfMuted).SetString(ToolPending)
	s.ToolCallError = lipgloss.NewStyle().Foreground(error).SetString(ToolError)
	s.ToolCallSuccess = lipgloss.NewStyle().Foreground(greenLight).SetString(ToolSuccess)
	// Cancelled uses muted tone but same glyph as pending
	s.ToolCallCancelled = s.Muted.SetString(ToolPending)
	s.EarlyStateMessage = s.Subtle.PaddingLeft(2)

	// Tool rendering styles
	// Icon palette follows the Codex-like neutral/green/rose state model.
	s.Tool.IconPending = base.Foreground(fgHalfMuted).SetString(ToolPending)
	s.Tool.IconSuccess = base.Foreground(greenLight).SetString(ToolSuccess)
	s.Tool.IconError = base.Foreground(error).SetString(ToolError)
	s.Tool.IconCancelled = s.Muted.SetString(ToolPending)

	// Tool names: warm accent emphasis
	s.Tool.NameNormal = base.Foreground(primary)
	s.Tool.NameNested = base.Foreground(fgHalfMuted)
	s.Tool.NameSuccess = base.Foreground(lipgloss.Color("#A9D7B0"))
	s.Tool.NameSuccessNested = base.Foreground(lipgloss.Color("#8FBC96"))

	s.Tool.ParamMain = s.Muted
	s.Tool.ParamKey = s.Subtle

	// Content rendering - prepared styles that accept width parameter
	codeBg := lipgloss.Color("#1A1624") // Darker purple-black for cleaner code blocks
	s.Tool.ContentLine = s.Base.Foreground(fgBase).Background(bgOverlay).Padding(0, 1)
	s.Tool.ContentTruncation = s.Muted.Background(bgOverlay).Padding(0, 1)
	s.Tool.ContentCodeLine = s.Base.Background(codeBg).Padding(0, 1)
	s.Tool.ContentCodeTruncation = s.Muted.Background(codeBg).Padding(0, 1)
	s.Tool.ContentCodeBg = codeBg
	s.Tool.Body = base.PaddingLeft(2)
	s.Tool.FileBlock = base.Background(codeBg).Padding(0, 1)

	// Deprecated - kept for backward compatibility
	s.Tool.ContentBg = s.Muted.Background(bgOverlay)
	s.Tool.ContentText = s.Muted
	s.Tool.ContentLineNumber = base.Foreground(fgHalfMuted).Background(bgSubtle).PaddingRight(1).PaddingLeft(1)

	s.Tool.StateWaiting = base.Foreground(fgSubtle)
	s.Tool.StateCancelled = base.Foreground(fgSubtle)

	s.Tool.ErrorTag = base.Foreground(error).Bold(true)
	s.Tool.ErrorMessage = base.Foreground(lipgloss.Color("#F8D4DD"))

	// Diff and multi-edit styles
	s.Tool.DiffTruncation = s.Muted.Background(bgBaseLighter).PaddingLeft(2)
	s.Tool.NoteTag = base.Padding(0, 1).Background(bgBaseLighter).Foreground(tertiary).Bold(true)
	s.Tool.NoteMessage = base.Foreground(fgHalfMuted)

	// Job header styles - warm accents
	s.Tool.JobIconPending = base.Foreground(fgHalfMuted)
	s.Tool.JobIconError = base.Foreground(error)
	s.Tool.JobIconSuccess = base.Foreground(greenLight)
	s.Tool.JobToolName = base.Foreground(primary)
	s.Tool.JobAction = base.Foreground(fgHalfMuted)
	s.Tool.JobPID = s.Muted
	s.Tool.JobDescription = s.Subtle
	s.Tool.BashLabel = base.Foreground(tertiary).Background(bgSubtle).Padding(0, 1).Bold(true)
	s.Tool.BashCommand = base.Foreground(fgBase).Background(bgSubtle).Padding(0, 1)
	s.Tool.BashOutputLabel = base.Foreground(greenLight).Bold(true)

	// Agent task styles - vibrant purple accent
	s.Tool.AgentTaskTag = base.Bold(true).Padding(0, 1).MarginLeft(2).Background(bgOverlay).Foreground(primary)
	s.Tool.AgentPrompt = s.Muted

	// Agentic fetch styles - purple accent
	s.Tool.AgenticFetchPromptTag = base.Bold(true).Padding(0, 1).MarginLeft(2).Background(bgOverlay).Foreground(tertiary)

	// Todo styles
	s.Tool.TodoRatio = base.Foreground(primary)
	s.Tool.TodoCompletedIcon = base.Foreground(greenLight)
	s.Tool.TodoInProgressIcon = base.Foreground(greenLight)
	s.Tool.TodoPendingIcon = base.Foreground(fgMuted)
	s.Tool.TodoFailedIcon = base.Foreground(error)
	s.Tool.TodoCanceledIcon = base.Foreground(secondary)

	// MCP styles
	s.Tool.MCPName = base.Foreground(primary)
	s.Tool.MCPToolName = base.Foreground(fgHalfMuted)
	s.Tool.MCPArrow = base.Foreground(tertiary).SetString(ArrowRightIcon)

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
	s.Tool.ResourceLoadedText = base.Foreground(secondary)
	s.Tool.ResourceLoadedIndicator = base.Foreground(greenDark)
	s.Tool.ResourceName = base
	s.Tool.MediaType = base
	s.Tool.ResourceSize = base.Foreground(fgMuted)
	s.Tool.SkillTag = base.Bold(true).Padding(0, 1).Background(bgSubtle).Foreground(tertiary)

	// Buttons
	s.ButtonFocus = lipgloss.NewStyle().Foreground(white).Background(primary)
	s.ButtonBlur = s.Base.Background(bgOverlay)

	// Borders
	s.BorderFocus = lipgloss.NewStyle().BorderForeground(borderFocus).Border(lipgloss.RoundedBorder()).Background(bgOverlay).Padding(1, 2)

	// Editor - use purple for YOLO mode, amber for off-state
	s.EditorPromptNormalFocused = lipgloss.NewStyle().Foreground(greenDark).SetString("::: ")
	s.EditorPromptNormalBlurred = s.EditorPromptNormalFocused.Foreground(fgMuted)
	yoloIconBg := primary // Purple for YOLO mode enabled
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
	s.EditorPromptYoloIconFocused = lipgloss.NewStyle().MarginRight(1).Foreground(white).Background(yoloIconBg).Bold(true).SetString(" ! ")
	s.EditorPromptYoloIconBlurred = s.EditorPromptYoloIconFocused.Foreground(charmtone.Pepper).Background(lipgloss.Color("#3d2a5c"))
	s.EditorPromptYoloDotsFocused = lipgloss.NewStyle().MarginRight(1).Foreground(primary).SetString(":::")
	s.EditorPromptYoloDotsBlurred = s.EditorPromptYoloDotsFocused.Foreground(lipgloss.Color("#7a5c9e"))

	s.RadioOn = s.HalfMuted.SetString(RadioOn)
	s.RadioOff = s.HalfMuted.SetString(RadioOff)

	// Logo colors
	s.LogoFieldColor = primary
	s.LogoTitleColorA = primary
	s.LogoTitleColorB = logoPink
	s.LogoCharmColor = tertiary
	s.LogoVersionColor = secondary

	// Section - use primary purple for stronger headers
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
	s.Files.Additions = s.Base.Foreground(greenLight).Bold(true)
	s.Files.Deletions = s.Base.Foreground(red).Bold(true)

	// Chat
	offWhite := lipgloss.Color("#E6E6E6")
	thinkingText := lipgloss.Color("#EDE6F7")
	thinkingAccent := lipgloss.Color("#D9B2F0")

	s.Chat.Message.NoContent = lipgloss.NewStyle().Foreground(fgBase)
	s.Chat.Message.UserBlurred = lipgloss.NewStyle().Foreground(primary).SetString("> ")
	s.Chat.Message.UserFocused = lipgloss.NewStyle().Foreground(primary).SetString("> ")
	s.Chat.Message.AssistantBlurred = lipgloss.NewStyle().Foreground(offWhite).Faint(true)
	s.Chat.Message.AssistantFocused = lipgloss.NewStyle().Foreground(offWhite).Faint(true)
	s.Chat.Message.Thinking = lipgloss.NewStyle().Foreground(thinkingText).MaxHeight(10)
	s.Chat.Message.ErrorTag = lipgloss.NewStyle().
		Foreground(error).
		Bold(true)
	s.Chat.Message.ErrorTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFD2E5"))
	s.Chat.Message.ErrorDetails = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D7C2CE")).
		PaddingLeft(2)

	// Message item styles
	s.Chat.Message.ToolCallFocused = s.Muted
	s.Chat.Message.ToolCallBlurred = s.Muted
	// No padding or border for compact tool calls within messages
	s.Chat.Message.ToolCallCompact = s.Muted
	s.Chat.Message.SectionHeader = s.Base
	s.Chat.Message.AssistantInfoIcon = s.Subtle
	s.Chat.Message.AssistantInfoModel = s.Muted
	s.Chat.Message.AssistantInfoProvider = s.Subtle
	s.Chat.Message.AssistantInfoDuration = s.Subtle

	s.Chat.Message.ThinkingBox = s.Base

	// Thinking section styles
	s.Chat.Message.ThinkingTruncationHint = lipgloss.NewStyle().Foreground(thinkingAccent)
	s.Chat.Message.ThinkingFooterTitle = lipgloss.NewStyle().Foreground(thinkingAccent)
	s.Chat.Message.ThinkingFooterDuration = lipgloss.NewStyle().Foreground(thinkingAccent)

	// Text selection.
	s.TextSelection = lipgloss.NewStyle().Foreground(charmtone.Salt).Background(tertiary)

	// Dialog styles
	s.Dialog.Title = base.Padding(0, 1).Foreground(primary)
	s.Dialog.TitleText = base.Foreground(primary)
	s.Dialog.TitleError = base.Foreground(red)
	s.Dialog.TitleAccent = base.Foreground(tertiary).Bold(true)
	dialogSurface := bgBase
	s.Dialog.View = base.Border(lipgloss.NormalBorder()).BorderForeground(primary).Background(dialogSurface).Padding(0, 1)
	s.Dialog.PrimaryText = base.Padding(0, 1).Foreground(primary)
	s.Dialog.SecondaryText = base.Padding(0, 1).Foreground(fgSubtle)
	s.Dialog.HelpView = base.Padding(0, 1).AlignHorizontal(lipgloss.Left)
	s.Dialog.Help.ShortKey = base.Foreground(fgMuted)
	s.Dialog.Help.ShortDesc = base.Foreground(fgSubtle)
	s.Dialog.Help.ShortSeparator = base.Foreground(primary)
	s.Dialog.Help.Ellipsis = base.Foreground(border)
	s.Dialog.Help.FullKey = base.Foreground(fgMuted)
	s.Dialog.Help.FullDesc = base.Foreground(fgSubtle)
	s.Dialog.Help.FullSeparator = base.Foreground(primary)
	s.Dialog.NormalItem = base.Padding(0, 1).Foreground(fgBase)
	s.Dialog.SelectedItem = base.Padding(0, 1).Background(primary).Foreground(white)
	s.Dialog.InputPrompt = base.Margin(1, 1)

	s.Dialog.List = base.Margin(0, 0, 1, 0)
	s.Dialog.ContentPanel = base.Background(dialogSurface).Foreground(fgBase).Padding(1, 2)
	s.Dialog.Spinner = base.Foreground(primary)
	s.Dialog.ScrollbarThumb = base.Foreground(primary)
	s.Dialog.ScrollbarTrack = base.Foreground(border)
	s.Dialog.View = base.Border(lipgloss.RoundedBorder()).BorderForeground(primary).Background(dialogSurface)

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
	attachmentIconStyle := base.Foreground(bgBase).Background(tertiary).Padding(0, 1)
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
