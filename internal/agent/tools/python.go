package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	"google.golang.org/genai"
)

//go:embed python.md
var pythonDescription []byte

const (
	PythonToolName = "python"

	// PythonExecutionTimeout is the maximum time for Python code execution.
	// Set to 2 minutes to allow Gemini to run multiple iterative executions.
	PythonExecutionTimeout = 2 * time.Minute

	// MaxPythonRetries is the maximum number of consecutive failures before giving up.
	// If Python execution fails this many times in a row, the tool will stop attempting.
	MaxPythonRetries = 3

	pythonExecutionAttempts = 2
)

type PythonToolParams struct {
	Prompt string `json:"prompt" description:"The exact computation, verification, or transformation task to run with Python."`
}

type PythonToolResponseMetadata struct {
	Prompt          string `json:"prompt"`
	Model           string `json:"model"`
	Text            string `json:"text,omitempty"`
	ExecutedCode    string `json:"executed_code,omitempty"`
	ExecutionOutput string `json:"execution_output,omitempty"`
	ExecutionTimeMs int64  `json:"execution_time_ms,omitempty"`
	Attempts        int    `json:"attempts,omitempty"`
}

func NewPythonTool(client *genai.Client, model string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		PythonToolName,
		string(pythonDescription),
		func(ctx context.Context, params PythonToolParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if client == nil {
				return fantasy.NewTextErrorResponse("python tool is not configured"), nil
			}

			prompt := strings.TrimSpace(params.Prompt)
			if prompt == "" {
				return fantasy.NewTextErrorResponse("prompt is required"), nil
			}

			var (
				text          string
				code          string
				output        string
				executionTime time.Duration
				attempts      int
			)

			for attempts = 1; attempts <= pythonExecutionAttempts; attempts++ {
				forceExecution := attempts > 1
				response, duration, err := executePythonPrompt(ctx, client, model, prompt, forceExecution)
				executionTime += duration
				if err != nil {
					if errorsIsTimeout(err) {
						msg := fmt.Sprintf("Python execution timed out after %v. The computation took too long or Gemini ran multiple iterative executions. Consider breaking the task into smaller steps.", PythonExecutionTimeout)
						return fantasy.NewTextErrorResponse(msg), nil
					}
					return fantasy.NewTextErrorResponse(fmt.Sprintf("gemini python execution failed: %s", err)), nil
				}

				text, code, output = parsePythonExecutionResponse(response)
				if code != "" || output != "" || attempts == pythonExecutionAttempts {
					break
				}
			}

			// Check for execution errors in output
			if strings.Contains(strings.ToLower(output), "error") ||
				strings.Contains(strings.ToLower(output), "exception") ||
				strings.Contains(strings.ToLower(output), "traceback") {
				// Execution failed - this will be tracked by the agent for retry
				return fantasy.WithResponseMetadata(
					fantasy.NewTextResponse("Python execution encountered an error:\n\n"+output),
					PythonToolResponseMetadata{
						Prompt:          prompt,
						Model:           model,
						Text:            text,
						ExecutedCode:    code,
						ExecutionOutput: output,
						ExecutionTimeMs: executionTime.Milliseconds(),
						Attempts:        attempts,
					},
				), nil
			}

			var sections []string
			if text != "" {
				sections = append(sections, text)
			}
			if output != "" {
				sections = append(sections, "Execution output:\n"+output)
			}
			if len(sections) == 0 {
				sections = append(sections, "Python environment returned no execution artifacts, but a direct answer was produced.")
			}

			metadata := PythonToolResponseMetadata{
				Prompt:          prompt,
				Model:           model,
				Text:            text,
				ExecutedCode:    code,
				ExecutionOutput: output,
				ExecutionTimeMs: executionTime.Milliseconds(),
				Attempts:        attempts,
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(strings.Join(sections, "\n\n")),
				metadata,
			), nil
		},
	)
}

func executePythonPrompt(
	ctx context.Context,
	client *genai.Client,
	model string,
	prompt string,
	forceExecution bool,
) (*genai.GenerateContentResponse, time.Duration, error) {
	execCtx, cancel := context.WithTimeout(ctx, PythonExecutionTimeout)
	defer cancel()

	startTime := time.Now()
	response, err := client.Models.GenerateContent(
		execCtx,
		model,
		genai.Text(buildPythonExecutionPrompt(prompt, forceExecution)),
		&genai.GenerateContentConfig{
			Tools: []*genai.Tool{
				{CodeExecution: &genai.ToolCodeExecution{}},
			},
			CandidateCount: 1,
		},
	)
	return response, time.Since(startTime), err
}

func buildPythonExecutionPrompt(task string, forceExecution bool) string {
	var b strings.Builder
	b.WriteString("Use Python code execution for this task. Generate and run Python code for the calculation, verification, or transformation before giving the final answer.\n")
	if forceExecution {
		b.WriteString("You must execute Python in this response. Do not answer from reasoning alone.\n")
	}
	b.WriteString("Return the computed result clearly after execution.\n\n")
	b.WriteString("Task:\n")
	b.WriteString(task)
	return b.String()
}

func errorsIsTimeout(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded")
}

func parsePythonExecutionResponse(response *genai.GenerateContentResponse) (string, string, string) {
	if response == nil || len(response.Candidates) == 0 || response.Candidates[0].Content == nil {
		return "", "", ""
	}

	var texts []string
	var codes []string
	var outputs []string

	for _, part := range response.Candidates[0].Content.Parts {
		if part == nil {
			continue
		}
		if text := strings.TrimSpace(part.Text); text != "" {
			texts = append(texts, text)
		}
		if part.ExecutableCode != nil {
			if code := strings.TrimSpace(part.ExecutableCode.Code); code != "" {
				codes = append(codes, code)
			}
		}
		if part.CodeExecutionResult != nil {
			if output := strings.TrimSpace(part.CodeExecutionResult.Output); output != "" {
				outputs = append(outputs, output)
			}
		}
	}

	return strings.Join(texts, "\n\n"), strings.Join(codes, "\n\n"), strings.Join(outputs, "\n\n")
}
