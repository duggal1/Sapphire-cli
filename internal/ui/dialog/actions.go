package dialog

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/sapphire/internal/commands"
	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/oauth"
	"github.com/charmbracelet/sapphire/internal/planmode"
	"github.com/charmbracelet/sapphire/internal/permission"
	"github.com/charmbracelet/sapphire/internal/session"
	"github.com/charmbracelet/sapphire/internal/ui/common"
	"github.com/charmbracelet/sapphire/internal/ui/util"
)

// ActionClose is a message to close the current dialog.
type ActionClose struct{}

// ActionQuit is a message to quit the application.
type ActionQuit = tea.QuitMsg

// ActionOpenDialog is a message to open a dialog.
type ActionOpenDialog struct {
	DialogID string
}

// ActionSelectSession is a message indicating a session has been selected.
type ActionSelectSession struct {
	Session session.Session
}

// ActionSelectModel is a message indicating a model has been selected.
type ActionSelectModel struct {
	Provider       catwalk.Provider
	Model          config.SelectedModel
	ModelType      config.SelectedModelType
	ReAuthenticate bool
}

// ActionSelectAgentMode is a message indicating an agent mode was selected.
type ActionSelectAgentMode struct {
	Mode config.AgentMode
}

// ActionImplementProposedPlan is a message to accept the proposed plan.
type ActionImplementProposedPlan struct{}

// ActionReviseProposedPlan is a message to revise the proposed plan.
type ActionReviseProposedPlan struct{}

// ActionExitPlanMode is a message to exit plan mode.
type ActionExitPlanMode struct{}

// ActionRespondUserInput is a message to respond to a request_user_input prompt.
type ActionRespondUserInput struct {
	RequestID string
	Response  planmode.Response
}

// Messages for commands
type (
	ActionNewSession        struct{}
	ActionToggleHelp        struct{}
	ActionToggleCompactMode struct{}
	ActionTogglePasteBlocks struct{}
	ActionToggleThinking    struct{}
	ActionTogglePills       struct{}
	ActionExternalEditor    struct{}
	ActionToggleYoloMode    struct{}
	ActionToggleGoogleGrounding struct{}
	// ActionInitializeProject is a message to initialize a project.
	ActionInitializeProject struct{}
	ActionSummarize         struct {
		SessionID string
	}
	// ActionSelectReasoningEffort is a message indicating a reasoning effort has been selected.
	ActionSelectReasoningEffort struct {
		Effort string
	}
	ActionPermissionResponse struct {
		Permission permission.PermissionRequest
		Action     PermissionAction
	}
	// ActionRunCustomCommand is a message to run a custom command.
	ActionRunCustomCommand struct {
		Content   string
		Arguments []commands.Argument
		Args      map[string]string // Actual argument values
	}
	// ActionRunMCPPrompt is a message to run a custom command.
	ActionRunMCPPrompt struct {
		Title       string
		Description string
		PromptID    string
		ClientID    string
		Arguments   []commands.Argument
		Args        map[string]string // Actual argument values
	}
	// ActionOpenMCPConfig opens the MCP configuration dialog.
	ActionOpenMCPConfig struct {
		Name   string
		Config config.MCPConfig
		IsNew  bool
	}
	// ActionSaveMCPConfig persists an MCP configuration.
	ActionSaveMCPConfig struct {
		Name         string
		OriginalName string
		Config       config.MCPConfig
	}
	// ActionDeleteMCPConfig removes an MCP configuration.
	ActionDeleteMCPConfig struct {
		Name string
	}
	// ActionToggleMCPConfig toggles MCP enabled/disabled.
	ActionToggleMCPConfig struct {
		Name string
	}
	// ActionOpenMCPTools opens the MCP tools list dialog.
	ActionOpenMCPTools struct {
		Name string
	}
	// ActionRefreshMCPServer refreshes MCP tools/prompts/resources.
	ActionRefreshMCPServer struct {
		Name string
	}
)

// Messages for API key input dialog.
type (
	ActionChangeAPIKeyState struct {
		State APIKeyInputState
	}
)

// Messages for OAuth2 device flow dialog.
type (
	// ActionInitiateOAuth is sent when the device auth is initiated
	// successfully.
	ActionInitiateOAuth struct {
		DeviceCode      string
		UserCode        string
		ExpiresIn       int
		VerificationURL string
		Interval        int
	}

	// ActionCompleteOAuth is sent when the device flow completes successfully.
	ActionCompleteOAuth struct {
		Token *oauth.Token
	}

	// ActionOAuthErrored is sent when the device flow encounters an error.
	ActionOAuthErrored struct {
		Error error
	}
)

// ActionCmd represents an action that carries a [tea.Cmd] to be passed to the
// Bubble Tea program loop.
type ActionCmd struct {
	Cmd tea.Cmd
}

// ActionFilePickerSelected is a message indicating a file has been selected in
// the file picker dialog.
type ActionFilePickerSelected struct {
	Path string
}

// Cmd returns a command that reads the file at path and sends a
// [message.Attachement] to the program.
func (a ActionFilePickerSelected) Cmd() tea.Cmd {
	path := a.Path
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		isFileLarge, err := common.IsFileTooBig(path, common.MaxAttachmentSize)
		if err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  fmt.Sprintf("unable to read the image: %v", err),
			}
		}
		if isFileLarge {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  "file too large, max 5MB",
			}
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  fmt.Sprintf("unable to read the image: %v", err),
			}
		}

		mimeBufferSize := min(512, len(content))
		mimeType := http.DetectContentType(content[:mimeBufferSize])
		fileName := filepath.Base(path)

		return message.Attachment{
			FilePath: path,
			FileName: fileName,
			MimeType: mimeType,
			Content:  content,
		}
	}
}
