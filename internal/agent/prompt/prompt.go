package prompt

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"
	_ "time/tzdata"

	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/home"
	"github.com/duggal1/Sapphire-cli/internal/shell"
)

const gitPromptTimeout = 750 * time.Millisecond

const (
	runtimeClockLocationNewYork      = "America/New_York"
	runtimeClockLocationSanFrancisco = "America/Los_Angeles"
	runtimeClockLocationKolkata      = "Asia/Kolkata"
)

// Prompt represents a template-based prompt generator.
type Prompt struct {
	name           string
	template       string
	now            func() time.Time
	platform       string
	workingDir     string
	planToolPrompt string
}

type PromptDat struct {
	Provider                       string
	Model                          string
	Config                         config.Config
	WorkingDir                     string
	IsGitRepo                      bool
	Platform                       string
	Date                           string
	RuntimeClock                   string
	RuntimeYear                    string
	RuntimeDate                    string
	RuntimeTime                    string
	RuntimeClockNewYork            string
	RuntimeClockSanFrancisco       string
	RuntimeClockKolkata            string
	GitStatus                      string
	ContextFiles                   []ContextFile
	PlanToolPrompt                 string
	HasApplicableAgentInstructions bool
}

type ContextFile struct {
	Path    string
	Content string
}

type Option func(*Prompt)

func WithTimeFunc(fn func() time.Time) Option {
	return func(p *Prompt) {
		p.now = fn
	}
}

func WithPlatform(platform string) Option {
	return func(p *Prompt) {
		p.platform = platform
	}
}

func WithWorkingDir(workingDir string) Option {
	return func(p *Prompt) {
		p.workingDir = workingDir
	}
}

func WithPlanToolPrompt(planToolPrompt string) Option {
	return func(p *Prompt) {
		p.planToolPrompt = planToolPrompt
	}
}

func runtimeClockInLocation(now time.Time, locationName string) string {
	loc, err := time.LoadLocation(locationName)
	if err != nil {
		return now.UTC().Format(time.RFC3339)
	}
	return now.In(loc).Format(time.RFC3339)
}

func NewPrompt(name, promptTemplate string, opts ...Option) (*Prompt, error) {
	p := &Prompt{
		name:     name,
		template: promptTemplate,
		now:      time.Now,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p, nil
}

func (p *Prompt) Build(ctx context.Context, provider, model string, cfg config.Config) (built string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			name := "<nil>"
			if p != nil {
				name = p.name
			}
			built = ""
			err = fmt.Errorf("building prompt %q failed: %v", name, recovered)
		}
	}()
	t, err := template.New(p.name).Parse(p.template)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}
	var sb strings.Builder
	d, err := p.promptData(ctx, provider, model, cfg)
	if err != nil {
		return "", err
	}
	if err := t.Execute(&sb, d); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return sb.String(), nil
}

func processFile(filePath string) *ContextFile {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	return &ContextFile{
		Path:    filePath,
		Content: string(content),
	}
}

func processContextPath(p string, cfg config.Config) []ContextFile {
	var contexts []ContextFile
	fullPath := p
	if !filepath.IsAbs(p) {
		fullPath = filepath.Join(cfg.WorkingDir(), p)
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return contexts
	}
	if info.IsDir() {
		filepath.WalkDir(fullPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				if result := processFile(path); result != nil {
					contexts = append(contexts, *result)
				}
			}
			return nil
		})
	} else {
		result := processFile(fullPath)
		if result != nil {
			contexts = append(contexts, *result)
		}
	}
	return contexts
}

// ExpandPath expands ~ and environment variables in file paths
func ExpandPath(path string, cfg config.Config) string {
	path = home.Long(path)
	// Handle environment variable expansion using the same pattern as config
	if strings.HasPrefix(path, "$") {
		if resolver := cfg.Resolver(); resolver != nil {
			if expanded, err := resolver.ResolveValue(path); err == nil {
				path = expanded
			}
		}
	}

	return path
}

func (p *Prompt) promptData(ctx context.Context, provider, model string, cfg config.Config) (PromptDat, error) {
	workingDir := cmp.Or(p.workingDir, cfg.WorkingDir())
	platform := cmp.Or(p.platform, runtime.GOOS)
	options := cfg.Options
	if options == nil {
		options = &config.Options{}
	}
	nowFn := p.now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()

	files := map[string][]ContextFile{}

	for _, pth := range options.ContextPaths {
		expanded := ExpandPath(pth, cfg)
		pathKey := strings.ToLower(expanded)
		if _, ok := files[pathKey]; ok {
			continue
		}
		content := processContextPath(expanded, cfg)
		files[pathKey] = content
	}

	isGit := isGitRepo(workingDir)
	data := PromptDat{
		Provider:                       provider,
		Model:                          model,
		Config:                         cfg,
		WorkingDir:                     filepath.ToSlash(workingDir),
		IsGitRepo:                      isGit,
		Platform:                       platform,
		Date:                           now.Format("1/2/2006"),
		RuntimeClock:                   now.Format(time.RFC3339),
		RuntimeYear:                    now.Format("2006"),
		RuntimeDate:                    now.Format("Monday, January 2, 2006"),
		RuntimeTime:                    now.Format("15:04:05 MST"),
		RuntimeClockNewYork:            runtimeClockInLocation(now, runtimeClockLocationNewYork),
		RuntimeClockSanFrancisco:       runtimeClockInLocation(now, runtimeClockLocationSanFrancisco),
		RuntimeClockKolkata:            runtimeClockInLocation(now, runtimeClockLocationKolkata),
		PlanToolPrompt:                 p.planToolPrompt,
		HasApplicableAgentInstructions: hasApplicableAgentInstructions(workingDir, cmp.Or(options.InitializeAs, "AGENTS.md")),
	}
	if isGit {
		var err error
		data.GitStatus, err = getGitStatus(ctx, workingDir)
		if err != nil {
			return PromptDat{}, err
		}
	}

	for _, contextFiles := range files {
		data.ContextFiles = append(data.ContextFiles, contextFiles...)
	}
	return data, nil
}

func hasApplicableAgentInstructions(workingDir, initializeAs string) bool {
	repoRoot := findPromptRepoRoot(workingDir)

	if strings.TrimSpace(initializeAs) != "" {
		if _, err := os.Stat(filepath.Join(repoRoot, initializeAs)); err == nil {
			return true
		}
	}

	candidates := []string{
		"AGENTS.md",
		"agents.md",
		"Agents.md",
		"agent.md",
	}

	for dir := filepath.Clean(workingDir); ; dir = filepath.Dir(dir) {
		for _, name := range candidates {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				return true
			}
		}
		if dir == repoRoot {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return false
}

func findPromptRepoRoot(workingDir string) string {
	if strings.TrimSpace(workingDir) == "" {
		return "."
	}
	dir := filepath.Clean(workingDir)
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Clean(workingDir)
		}
		dir = parent
	}
}

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func getGitStatus(ctx context.Context, dir string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitPromptTimeout)
	defer cancel()

	sh := shell.NewShell(&shell.Options{
		WorkingDir: dir,
	})
	branch, err := getGitBranch(ctx, sh)
	if err != nil {
		return "", nil
	}
	status, err := getGitStatusSummary(ctx, sh)
	if err != nil {
		return branch, nil
	}
	commits, err := getGitRecentCommits(ctx, sh)
	if err != nil {
		return branch + status, nil
	}
	return branch + status + commits, nil
}

func getGitBranch(ctx context.Context, sh *shell.Shell) (string, error) {
	out, _, err := sh.Exec(ctx, "git branch --show-current 2>/dev/null")
	if err != nil {
		return "", nil
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", nil
	}
	return fmt.Sprintf("Current branch: %s\n", out), nil
}

func getGitStatusSummary(ctx context.Context, sh *shell.Shell) (string, error) {
	out, _, err := sh.Exec(ctx, "git status --short --untracked-files=no 2>/dev/null")
	if err != nil {
		return "", nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > 20 {
		lines = lines[:20]
	}
	out = strings.TrimSpace(strings.Join(lines, "\n"))
	if out == "" {
		return "Status: clean\n", nil
	}
	return fmt.Sprintf("Status:\n%s\n", out), nil
}

func getGitRecentCommits(ctx context.Context, sh *shell.Shell) (string, error) {
	out, _, err := sh.Exec(ctx, "git log --oneline --no-decorate -n 3 2>/dev/null")
	if err != nil || out == "" {
		return "", nil
	}
	out = strings.TrimSpace(out)
	return fmt.Sprintf("Recent commits:\n%s\n", out), nil
}

func (p *Prompt) Name() string {
	return p.name
}
