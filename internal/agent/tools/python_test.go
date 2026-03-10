package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"
)

func TestParsePythonExecutionResponse(t *testing.T) {
	t.Parallel()

	response := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{Text: "I will calculate the totals."},
						{ExecutableCode: &genai.ExecutableCode{Code: "print(1 + 1)", Language: genai.LanguagePython}},
						{CodeExecutionResult: &genai.CodeExecutionResult{Output: "2", Outcome: genai.OutcomeOK}},
						{Text: "The total is 2."},
					},
				},
			},
		},
	}

	text, code, output := parsePythonExecutionResponse(response)
	assert.Equal(t, "I will calculate the totals.\n\nThe total is 2.", text)
	assert.Equal(t, "print(1 + 1)", code)
	assert.Equal(t, "2", output)
}

func TestBuildPythonExecutionPrompt(t *testing.T) {
	t.Parallel()

	prompt := buildPythonExecutionPrompt("sum the values", false)
	assert.Contains(t, prompt, "Use Python code execution for this task.")
	assert.Contains(t, prompt, "Task:\nsum the values")
	assert.NotContains(t, prompt, "You must execute Python in this response.")

	forced := buildPythonExecutionPrompt("sum the values", true)
	assert.Contains(t, forced, "You must execute Python in this response.")
}
