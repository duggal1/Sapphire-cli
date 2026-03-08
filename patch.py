import re
import os

with open('internal/agent/tools/view.go', 'r') as f:
    content = f.read()

content = content.replace('ViewToolName     = "view"', 'ViewToolName     = "agentic_view"')

old_params = """type ViewParams struct {
	FilePath string `json:"file_path" description:"The path to the file to read"`
	Offset   int    `json:"offset,omitempty" description:"The line number to start reading from (0-based)"`
	Limit    int    `json:"limit,omitempty" description:"The number of lines to read (defaults to 2000)"`
}"""

new_params = """type ViewParams struct {
	FilePaths []string `json:"file_paths" description:"The paths to the files to read. Max concurrent reads will apply."`
	FilePath  string   `json:"file_path,omitempty" description:"The path to the file to read (legacy single file)"`
	Offset    int      `json:"offset,omitempty" description:"The line number to start reading from (0-based, applies to single file only)"`
	Limit     int      `json:"limit,omitempty" description:"The number of lines to read (defaults to 2000, applies to single file only)"`
}"""

content = content.replace(old_params, new_params)

with open('internal/agent/tools/view.go', 'w') as f:
    f.write(content)
