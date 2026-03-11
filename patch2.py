import re

with open('internal/agent/tools/view.go', 'r') as f:
    content = f.read()

# Make NewViewTool accept maxConcurrent
old_signature = """func NewViewTool(
	lspManager *lsp.Manager,
	permissions permission.Service,
	filetracker filetracker.Service,
	workingDir string,
	skillsPaths ...string,
) fantasy.AgentTool {"""

new_signature = """func NewViewTool(
	lspManager *lsp.Manager,
	permissions permission.Service,
	filetracker filetracker.Service,
	workingDir string,
	maxConcurrent int,
	skillsPaths ...string,
) fantasy.AgentTool {"""

content = content.replace(old_signature, new_signature)

new_func_body = """		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			filePaths := params.FilePaths
			if len(filePaths) == 0 && params.FilePath != "" {
				filePaths = []string{params.FilePath}
			}
			if len(filePaths) == 0 {
				return fantasy.NewTextErrorResponse("file_paths is required"), nil
			}

			if len(filePaths) > maxConcurrent {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Too many files requested. Maximum allowed is %d", maxConcurrent)), nil
			}

			type fileResult struct {
				content string
				mimeType string
				isImage bool
				err error
			}

			results := make([]fileResult, len(filePaths))
			errChan := make(chan error, 1)

			// Simple concurrent execution without external dependencies
			done := make(chan bool)
			go func() {
				for i, fp := range filePaths {
					i := i
					fp := fp
					go func() {
						res := fileResult{}
						
						// Handle relative paths
						filePath := filepathext.SmartJoin(workingDir, fp)

						// Check if file is outside working directory and request permission if needed
						absWorkingDir, err := filepath.Abs(workingDir)
						if err != nil {
							res.err = fmt.Errorf("error resolving working directory: %w", err)
							results[i] = res
							return
						}

						absFilePath, err := filepath.Abs(filePath)
						if err != nil {
							res.err = fmt.Errorf("error resolving file path: %w", err)
							results[i] = res
							return
						}

						relPath, err := filepath.Rel(absWorkingDir, absFilePath)
						isOutsideWorkDir := err != nil || strings.HasPrefix(relPath, "..")
						isSkillFile := isInSkillsPath(absFilePath, skillsPaths)

						sessionID := GetSessionFromContext(ctx)
						if sessionID == "" {
							res.err = fmt.Errorf("session ID is required for accessing files outside working directory")
							results[i] = res
							return
						}

						// Request permission for files outside working directory, unless it's a skill file.
						if isOutsideWorkDir && !isSkillFile {
							granted, permReqErr := permissions.Request(ctx,
								permission.CreatePermissionRequest{
									SessionID:   sessionID,
									Path:        absFilePath,
									ToolCallID:  call.ID,
									ToolName:    ViewToolName,
									Action:      "read",
									Description: fmt.Sprintf("Read file outside working directory: %s", absFilePath),
									Params:      ViewPermissionsParams{FilePath: fp, Offset: params.Offset, Limit: params.Limit},
								},
							)
							if permReqErr != nil {
								res.err = permReqErr
								results[i] = res
								return
							}
							if !granted {
								res.err = permission.ErrorPermissionDenied
								results[i] = res
								return
							}
						}

						// Check if file exists
						fileInfo, err := os.Stat(filePath)
						if err != nil {
							res.err = fmt.Errorf("error accessing file: %w", err)
							results[i] = res
							return
						}

						// Check if it's a directory
						if fileInfo.IsDir() {
							res.err = fmt.Errorf("Path is a directory, not a file: %s", filePath)
							results[i] = res
							return
						}

						// Based on the specifications we should not limit the skills read.
						if !isSkillFile && fileInfo.Size() > MaxReadSize {
							res.err = fmt.Errorf("File is too large (%d bytes). Maximum size is %d bytes", fileInfo.Size(), MaxReadSize)
							results[i] = res
							return
						}

						limit := params.Limit
						if limit <= 0 {
							if isSkillFile {
								limit = 1000000 // Effectively no limit for skill files
							} else {
								limit = DefaultReadLimit
							}
						}

						isSupportedImage, mimeType := getImageMimeType(filePath)
						if isSupportedImage {
							if !GetSupportsImagesFromContext(ctx) {
								res.err = fmt.Errorf("This model does not support image data.")
								results[i] = res
								return
							}

							imageData, readErr := os.ReadFile(filePath)
							if readErr != nil {
								res.err = fmt.Errorf("error reading image file: %w", readErr)
								results[i] = res
								return
							}

							encoded := base64.StdEncoding.EncodeToString(imageData)
							res.isImage = true
							res.mimeType = mimeType
							res.content = encoded
							results[i] = res
							return
						}

						// Read the file content
						content, hasMore, err := readTextFile(filePath, params.Offset, limit)
						if err != nil {
							res.err = fmt.Errorf("error reading file: %w", err)
							results[i] = res
							return
						}
						if !utf8.ValidString(content) {
							res.err = fmt.Errorf("File content is not valid UTF-8")
							results[i] = res
							return
						}

						openInLSPs(ctx, lspManager, filePath)
						waitForLSPDiagnostics(ctx, lspManager, filePath, 300*time.Millisecond)
						output := "<file>\n"
						output += addLineNumbers(content, params.Offset+1)
						if hasMore {
							output += fmt.Sprintf("\n... (File truncated after %d lines. Use offset to read more.) ...", limit)
						}
						output += "\n</file>"
						
						res.content = output
						
						// Register file with filetracker
						if !isSkillFile {
							absPath, err := filepath.Abs(filePath)
							if err == nil {
								filetracker.AddRead(sessionID, absPath)
							}
						}
						results[i] = res
					}()
				}
				
				// Just wait a tiny bit to allow the go routines to finish
				// Wait this is bad, let's use a channel to collect completions
			}()"""

# Use regex to replace the function body
pattern = re.compile(r'func\(ctx context\.Context, params ViewParams, call fantasy\.ToolCall\) \(fantasy\.ToolResponse, error\) \{.*?\n\t\t\}\,', re.DOTALL)

with open('internal/agent/tools/view.go', 'r') as f:
    orig = f.read()

# I will write a better sync logic using WaitGroup
new_func_body = """func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			filePaths := params.FilePaths
			if len(filePaths) == 0 && params.FilePath != "" {
				filePaths = []string{params.FilePath}
			}
			if len(filePaths) == 0 {
				return fantasy.NewTextErrorResponse("file_paths is required"), nil
			}

			if len(filePaths) > maxConcurrent {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Too many files requested. Maximum allowed is %d", maxConcurrent)), nil
			}

			type fileResult struct {
				content string
				mimeType string
				isImage bool
				err error
				errText string
			}

			results := make([]fileResult, len(filePaths))
			
			errChan := make(chan error, len(filePaths))
			done := make(chan struct{})
			
			go func() {
				// Use a simple worker pattern or wait group
				var wg sync.WaitGroup
				for i, fp := range filePaths {
					wg.Add(1)
					go func(idx int, path string) {
						defer wg.Done()
						res := fileResult{}
						
						// Handle relative paths
						filePath := filepathext.SmartJoin(workingDir, path)

						absWorkingDir, err := filepath.Abs(workingDir)
						if err != nil {
							res.errText = fmt.Sprintf("error resolving working directory: %v", err)
							results[idx] = res
							return
						}

						absFilePath, err := filepath.Abs(filePath)
						if err != nil {
							res.errText = fmt.Sprintf("error resolving file path: %v", err)
							results[idx] = res
							return
						}

						relPath, err := filepath.Rel(absWorkingDir, absFilePath)
						isOutsideWorkDir := err != nil || strings.HasPrefix(relPath, "..")
						isSkillFile := isInSkillsPath(absFilePath, skillsPaths)

						sessionID := GetSessionFromContext(ctx)
						if sessionID == "" {
							res.errText = "session ID is required for accessing files outside working directory"
							results[idx] = res
							return
						}

						if isOutsideWorkDir && !isSkillFile {
							granted, permReqErr := permissions.Request(ctx,
								permission.CreatePermissionRequest{
									SessionID:   sessionID,
									Path:        absFilePath,
									ToolCallID:  call.ID,
									ToolName:    ViewToolName,
									Action:      "read",
									Description: fmt.Sprintf("Read file outside working directory: %s", absFilePath),
									Params:      ViewPermissionsParams{FilePath: path, Offset: params.Offset, Limit: params.Limit},
								},
							)
							if permReqErr != nil {
								res.err = permReqErr
								results[idx] = res
								return
							}
							if !granted {
								res.errText = permission.ErrorPermissionDenied.Error()
								results[idx] = res
								return
							}
						}

						fileInfo, err := os.Stat(filePath)
						if err != nil {
							if os.IsNotExist(err) {
								res.errText = fmt.Sprintf("File not found: %s", filePath)
							} else {
								res.errText = fmt.Sprintf("error accessing file: %v", err)
							}
							results[idx] = res
							return
						}

						if fileInfo.IsDir() {
							res.errText = fmt.Sprintf("Path is a directory, not a file: %s", filePath)
							results[idx] = res
							return
						}

						if !isSkillFile && fileInfo.Size() > MaxReadSize {
							res.errText = fmt.Sprintf("File is too large (%d bytes). Maximum size is %d bytes", fileInfo.Size(), MaxReadSize)
							results[idx] = res
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

						isSupportedImage, mimeType := getImageMimeType(filePath)
						if isSupportedImage {
							if !GetSupportsImagesFromContext(ctx) {
								res.errText = "This model does not support image data."
								results[idx] = res
								return
							}

							imageData, readErr := os.ReadFile(filePath)
							if readErr != nil {
								res.errText = fmt.Sprintf("error reading image file: %v", readErr)
								results[idx] = res
								return
							}

							encoded := base64.StdEncoding.EncodeToString(imageData)
							res.isImage = true
							res.mimeType = mimeType
							res.content = encoded
							results[idx] = res
							return
						}

						content, hasMore, err := readTextFile(filePath, params.Offset, limit)
						if err != nil {
							res.errText = fmt.Sprintf("error reading file: %v", err)
							results[idx] = res
							return
						}
						if !utf8.ValidString(content) {
							res.errText = "File content is not valid UTF-8"
							results[idx] = res
							return
						}

						openInLSPs(ctx, lspManager, filePath)
						waitForLSPDiagnostics(ctx, lspManager, filePath, 300*time.Millisecond)
						output := "<file path=\\""+path+"\\">\n"
						output += addLineNumbers(content, params.Offset+1)
						if hasMore {
							output += fmt.Sprintf("\n... (File truncated after %d lines. Use offset to read more.) ...", limit)
						}
						output += "\n</file>"
						
						res.content = output
						
						if !isSkillFile {
							absPath, err := filepath.Abs(filePath)
							if err == nil {
								filetracker.AddRead(sessionID, absPath)
							}
						}
						results[idx] = res
					}(i, fp)
				}
				wg.Wait()
				close(done)
			}()

			select {
			case <-ctx.Done():
				return fantasy.ToolResponse{}, ctx.Err()
			case <-done:
				// all done
			}

			// check for hard errors (e.g. context/permission errors)
			for _, res := range results {
				if res.err != nil {
					return fantasy.ToolResponse{}, res.err
				}
			}

			// Format results
			var fullContent strings.Builder
			for i, res := range results {
				if res.errText != "" {
					fullContent.WriteString(fmt.Sprintf("Error reading %s: %s\n\n", filePaths[i], res.errText))
				} else if res.isImage {
					// if there is one image among files, we might need to handle it differently, 
					// but for now let's just return a mixed text/image response if possible.
					// Actually, the current API only supports NewImageResponse for a single image,
					// and NewTextResponse for text. Let's fallback to returning just the image if it's the only one.
					if len(results) == 1 {
						return fantasy.NewImageResponse([]byte(res.content), res.mimeType), nil
					}
					fullContent.WriteString(fmt.Sprintf("Image file %s loaded (can only be viewed if requested individually)\n\n", filePaths[i]))
				} else {
					fullContent.WriteString(res.content)
					fullContent.WriteString("\n\n")
				}
			}
			
			// Set resource metadata if single file for UI
			meta := ViewResponseMetadata{
				Content: fullContent.String(),
			}
			
			if len(filePaths) == 1 {
				absFilePath, _ := filepath.Abs(filepathext.SmartJoin(workingDir, filePaths[0]))
				meta.FilePath = filePaths[0]
				isSkillFile := isInSkillsPath(absFilePath, skillsPaths)
				if isSkillFile {
					meta.ResourceType = ViewResourceSkill
					meta.ResourceName = filepath.Base(filepath.Dir(absFilePath))
					meta.ResourceDescription = "Skill instructions"
				}
			} else {
				meta.FilePath = fmt.Sprintf("%d files", len(filePaths))
			}

			return fantasy.NewTextResponseWithMetadata(fullContent.String(), meta), nil
		},"""

orig = re.sub(r'func\(ctx context\.Context, params ViewParams, call fantasy\.ToolCall\) \(fantasy\.ToolResponse, error\) \{.*?\n\t\t\}\,', new_func_body + '\n\t\t,', orig, flags=re.DOTALL)

with open('internal/agent/tools/view.go', 'w') as f:
    f.write(orig)
