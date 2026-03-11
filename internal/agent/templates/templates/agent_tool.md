Launch a new agent that has the same tool surface as the main CLI agent, including file operations, bash, web fetch/search, MCP tools, and code intelligence tools when they are enabled in the current Crush configuration.

<usage>
- If you are searching for a keyword like "config" or "logger", or for questions like "which file does X?", the Agent tool is strongly recommended
- Use this tool proactively for complex tasks. If the task spans multiple files, packages, subsystems, or requires codebase discovery, launch sub-agents immediately instead of trying to read everything yourself first.
- For large or ambiguous work, launch 2-5 sub-agents in parallel with narrow goals such as: relevant file discovery, dependency tracing, risk identification, test impact, API/data flow mapping, implementation, or validation.
- Sub-agents can use the same built-in Crush capabilities that are enabled for the current session, including `agentic_fetch` for web research.
- If you want to read a specific file path, use the View or GlobTool tool instead of the Agent tool, to find the match more quickly
- If you are searching for a specific class definition like "class Foo", use the GlobTool tool instead, to find the match more quickly
</usage>

<usage_notes>
1. Launch multiple agents concurrently whenever possible, to maximize performance; to do that, use a single message with multiple tool uses
2. When the agent is done, it will return a single message back to you. The result returned by the agent is not visible to the user. To show the user the result, you should send a text message back to the user with a concise summary of the result.
3. Each agent invocation is stateless. You will not be able to send additional messages to the agent, nor will the agent be able to communicate with you outside of its final report. Therefore, your prompt should contain a highly detailed task description for the agent to perform autonomously and you should specify exactly what information the agent should return back to you in its final and only message to you.
4. Ask sub-agents to return only the information you need: absolute file paths, concise findings, risks, and the minimum evidence needed to support their conclusion.
5. Sub-agents may investigate, edit, run bash, fetch web content, and validate results when the task requires it.
6. If the user asks for a repo-wide review, architecture analysis, complex debugging, or a multi-step implementation, default to using sub-agents unless there is an obvious single-file path.
7. The agent's outputs should generally be trusted
</usage_notes>
