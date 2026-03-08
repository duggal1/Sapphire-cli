Launch a new sub-agent that has access to an expansive set of tools including Agentic View, Agentic Fetch, Bash (with background terminal capability), GlobTool, GrepTool, LS, and standard View. Use the Agent tool to delegate complex search, exploration, and execution tasks.

<usage>
- If you are searching for a keyword like "config" or "logger", or for questions like "which file does X?", the Agent tool is strongly recommended.
- You can delegate complex multi-step analysis or codebase exploration.
</usage>

<usage_notes>
1. All sub-agents must be spawned and run in parallel. The main agent must launch multiple sub-agents simultaneously when dealing with multiple tasks. No sub-agent should block another — all sub-agents run concurrently. You must never wait for one sub-agent to finish before launching the next. To do this, issue multiple tool calls in a single message.
2. When the agent is done, it will return a single message back to you. The result returned by the agent is not visible to the user. To show the user the result, you should send a text message back to the user with a concise summary of the result.
3. Each agent invocation is stateless. You will not be able to send additional messages to the agent, nor will the agent be able to communicate with you outside of its final report. Therefore, your prompt should contain a highly detailed task description for the agent to perform autonomously and you should specify exactly what information the agent should return back to you in its final and only message to you.
4. The agent's outputs should generally be trusted.
5. IMPORTANT: Sub-agents have full capability to read files (Agentic View), search the web (Agentic Fetch), run terminal commands (Bash), and modify code (Agentic Edit/Write). Give them clear, scoped boundaries so they don't step on each other's toes when modifying files.
</usage_notes>
