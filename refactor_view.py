import re

with open('internal/agent/tools/view.go', 'r') as f:
    content = f.read()

# Replace NewViewTool signature
old_sig = """func NewViewTool(
	lspManager *lsp.Manager,
	permissions permission.Service,
	filetracker filetracker.Service,
	workingDir string,
	skillsPaths ...string,
) fantasy.AgentTool {"""

new_sig = """func NewViewTool(
	lspManager *lsp.Manager,
	permissions permission.Service,
	filetracker filetracker.Service,
	workingDir string,
	maxConcurrent int,
	skillsPaths ...string,
) fantasy.AgentTool {"""

content = content.replace(old_sig, new_sig)

# We need to completely rewrite the body of NewViewTool func.
# Find the start of the body func
start_idx = content.find("func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {")

# Find the matching closing brace of NewViewTool
# The end of the function is just before the definition of addLineNumbers
end_idx = content.find("func addLineNumbers(", start_idx)
if end_idx != -1:
    # Need to go back to find the closing brace of NewViewTool
    # It ends with `})` then `}`
    # Let's just find `})` then `}` before `func addLineNumbers`
    end_brace_idx = content.rfind("}", start_idx, end_idx)
    # The whole body to replace:
    body_to_replace = content[start_idx:end_brace_idx+1]

new_body = """func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			filePaths := params.FilePaths
			if params.FilePath != "" {
				filePaths = append(filePaths, params.FilePath)
			}
			
			if len(filePaths) == 0 {
				return fantasy.NewTextErrorResponse("file_paths or file_path is required"), nil
			}

			if maxConcurrent <= 0 {
				maxConcurrent = 1
			}

			// Deduplicate file paths
			uniquePaths := make([]string, 0, len(filePaths))
			seenPaths := make(map[string]bool)
			for _, p := range filePaths {
				if p == "" {
					continue
				}
				if !seenPaths[p] {
					seenPaths[p] = true
					uniquePaths = append(uniquePaths, p)
				}
			}

			if len(uniquePaths) == 0 {
				return fantasy.NewTextErrorResponse("no valid file paths provided"), nil
			}

			var wg sync.WaitGroup
			var mu sync.Mutex
			
			sem := make(chan struct{}, maxConcurrent)
			
			type fileResult struct {
				filePath string
				output   string
				err      error
				meta     ViewResponseMetadata
			}
			
			results := make([]fileResult, len(uniquePaths))
			
			sessionID := GetSessionFromContext(ctx)

			for i, p := range uniquePaths {
				wg.Add(1)
				go func(idx int, filePath string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					
					// Handle relative paths
					fullPath := filepathext.SmartJoin(workingDir, filePath)

					// Check if file is outside working directory
					absWorkingDir, err := filepath.Abs(workingDir)
					if err != nil {
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("error resolving working directory: %w", err)}
						return
					}

					absFilePath, err := filepath.Abs(fullPath)
					if err != nil {
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("error resolving file path: %w", err)}
						return
					}

					relPath, err := filepath.Rel(absWorkingDir, absFilePath)
					isOutsideWorkDir := err != nil || strings.HasPrefix(relPath, "..")
					isSkillFile := isInSkillsPath(absFilePath, skillsPaths)

					if sessionID == "" && (isOutsideWorkDir && !isSkillFile) {
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("session ID is required for accessing files outside working directory")}
						return
					}

					if isOutsideWorkDir && !isSkillFile {
						mu.Lock()
						granted, permReqErr := permissions.Request(ctx,
							permission.CreatePermissionRequest{
								SessionID:   sessionID,
								Path:        absFilePath,
								ToolCallID:  call.ID,
								ToolName:    ViewToolName,
								Action:      "read",
								Description: fmt.Sprintf("Read file outside working directory: %s", absFilePath),
								Params:      ViewPermissionsParams{FilePath: filePath, Offset: params.Offset, Limit: params.Limit},
							},
						)
						mu.Unlock()
						
						if permReqErr != nil {
							results[idx] = fileResult{filePath: filePath, err: permReqErr}
							return
						}
						if !granted {
							results[idx] = fileResult{filePath: filePath, err: permission.ErrorPermissionDenied}
							return
						}
					}

					fileInfo, err := os.Stat(fullPath)
					if err != nil {
						if os.IsNotExist(err) {
							dir := filepath.Dir(fullPath)
							base := filepath.Base(fullPath)
							dirEntries, dirErr := os.ReadDir(dir)
							var suggestions []string
							if dirErr == nil {
								for _, entry := range dirEntries {
									if strings.Contains(strings.ToLower(entry.Name()), strings.ToLower(base)) ||
										strings.Contains(strings.ToLower(base), strings.ToLower(entry.Name())) {
										suggestions = append(suggestions, filepath.Join(dir, entry.Name()))
										if len(suggestions) >= 3 {
											break
										}
									}
								}
							}
							if len(suggestions) > 0 {
								results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("File not found: %s\n\nDid you mean one of these?\n%s", filePath, strings.Join(suggestions, "\n"))}
							} else {
								results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("File not found: %s", filePath)}
							}
							return
						}
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("error accessing file %s: %w", filePath, err)}
						return
					}

					if fileInfo.IsDir() {
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("Path is a directory, not a file: %s", filePath)}
						return
					}

					if !isSkillFile && fileInfo.Size() > MaxReadSize {
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("File %s is too large (%d bytes). Maximum size is %d bytes", filePath, fileInfo.Size(), MaxReadSize)}
						return
					}

					limit := params.Limit
					if limit <= 0 {
						if isSkillFile {
							limit = 1000000
						} else {
							limit = DefaultReadLimit
						}
					}

					isSupportedImage, mimeType := getImageMimeType(fullPath)
					if isSupportedImage {
						if !GetSupportsImagesFromContext(ctx) {
							modelName := GetModelNameFromContext(ctx)
							results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("This model (%s) does not support image data for file %s", modelName, filePath)}
							return
						}

						imageData, readErr := os.ReadFile(fullPath)
						if readErr != nil {
							results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("error reading image file %s: %w", filePath, readErr)}
							return
						}

						encoded := base64.StdEncoding.EncodeToString(imageData)
						// For image, we can just return it if it's the only file requested.
						// If there are multiple, returning base64 in text might be huge.
						// We will just store it as output text, though for real image responses
						// we usually use fantasy.NewImageResponse. For multiple files, we fallback to text.
						if len(uniquePaths) == 1 {
							results[idx] = fileResult{filePath: filePath, output: "IMAGE:" + mimeType + ":" + encoded}
							return
						}
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("Image file %s cannot be read in parallel with other files", filePath)}
						return
					}

					content, hasMore, err := readTextFile(fullPath, params.Offset, limit)
					if err != nil {
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("error reading file %s: %w", filePath, err)}
						return
					}
					if !utf8.ValidString(content) {
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("File content %s is not valid UTF-8", filePath)}
						return
					}

					mu.Lock()
					openInLSPs(ctx, lspManager, fullPath)
					waitForLSPDiagnostics(ctx, lspManager, fullPath, 300*time.Millisecond)
					mu.Unlock()

					output := fmt.Sprintf("<file path=\"%s\">\n", filePath)
					output += addLineNumbers(content, params.Offset+1)
					if hasMore {
						output += fmt.Sprintf("\n\n(File has more lines. Use 'offset' parameter to read beyond line %d)", params.Offset+len(strings.Split(content, "\n")))
					}
					output += "\n</file>\n"
					output += getDiagnostics(fullPath, lspManager)
					
					mu.Lock()
					filetracker.RecordRead(ctx, sessionID, fullPath)
					mu.Unlock()

					meta := ViewResponseMetadata{
						FilePath: fullPath,
						Content:  content,
					}
					if isSkillFile {
						if skill, err := skills.Parse(fullPath); err == nil {
							meta.ResourceType = ViewResourceSkill
							meta.ResourceName = skill.Name
							meta.ResourceDescription = skill.Description
						}
					}

					results[idx] = fileResult{filePath: filePath, output: output, meta: meta}
				}(i, p)
			}

			wg.Wait()

			// Check if single image response
			if len(uniquePaths) == 1 && results[0].output != "" && strings.HasPrefix(results[0].output, "IMAGE:") {
				parts := strings.SplitN(results[0].output, ":", 3)
				return fantasy.NewImageResponse([]byte(parts[2]), parts[1]), nil
			}

			var finalOutput strings.Builder
			var allErrors []string
			var finalMeta ViewResponseMetadata

			for i, res := range results {
				if res.err != nil {
					allErrors = append(allErrors, res.err.Error())
				} else {
					finalOutput.WriteString(res.output)
					finalOutput.WriteString("\n")
					// Use the first successful file's meta for legacy compatibility
					if i == 0 || finalMeta.FilePath == "" {
						finalMeta = res.meta
					}
				}
			}

			if len(allErrors) > 0 {
				if len(allErrors) == len(uniquePaths) {
					// All failed
					return fantasy.NewTextErrorResponse(strings.Join(allErrors, "\n\n")), nil
				}
				// Partial success
				finalOutput.WriteString("\nErrors encountered:\n")
				finalOutput.WriteString(strings.Join(allErrors, "\n"))
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(finalOutput.String()),
				finalMeta,
			), nil
		})"""

content = content.replace(body_to_replace, new_body)

with open('internal/agent/tools/view.go', 'w') as f:
    f.write(content)
