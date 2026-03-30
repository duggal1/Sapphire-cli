# Codex Plan Mode Architecture

**Location:** `/Users/harshitduggal/workspace/Sapphire-cli/codex/codex-rs`

**Last Updated:** Based on analysis of codex-rs codebase

---

## 1. Overview

Plan Mode is a **collaboration mode** in Codex that enables conversational planning before implementation. It is designed to produce **decision-complete** plans that can be handed off to another engineer or agent for immediate implementation.

### Key Characteristics

- **Conversational Planning**: 3-phase approach (Ground → Intent → Implementation)
- **Non-Mutating by Default**: Only allows read-only, plan-improving actions
- **Structured Output**: Uses `<proposed_plan>` XML blocks for machine-readable plan extraction
- **Tool Restrictions**: `update_plan` tool is explicitly disabled in Plan Mode
- **Request User Input**: The `request_user_input` tool is available for clarifying questions

---

## 2. Architecture Components

### 2.1 High-Level Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    User Interface Layer                          │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ /plan cmd   │  │ Mode Switch  │  │ Implementation Prompt│  │
│  └─────────────┘  └──────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                 Collaboration Mode System                        │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ ModeKind    │  │ Mode Mask    │  │ Mode Instructions    │  │
│  └─────────────┘  └──────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Streaming Parser Layer                         │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ Assistant   │  │ ProposedPlan │  │ Citation Parser      │  │
│  │ Text Parser │  │ Parser       │  │                      │  │
│  └─────────────┘  └──────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      TUI Rendering                               │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ Plan Style  │  │ History Cell │  │ Notification Prompt  │  │
│  └─────────────┘  └──────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 3. Core Data Structures

### 3.1 ModeKind Enum

**Location:** `protocol/src/config_types.rs`

```rust
pub enum ModeKind {
    Default,
    Plan,
    PairProgramming,  // hidden
    Execute,          // hidden
}

impl ModeKind {
    pub const fn display_name(self) -> &'static str {
        match self {
            Self::Plan => "Plan",
            Self::Default => "Default",
            Self::PairProgramming => "Pair Programming",
            Self::Execute => "Execute",
        }
    }

    pub const fn is_tui_visible(self) -> bool {
        matches!(self, Self::Plan | Self::Default)
    }

    pub const fn allows_request_user_input(self) -> bool {
        matches!(self, Self::Plan)
    }
}
```

**Key Properties:**
- Only `Plan` and `Default` modes are visible in TUI
- Only `Plan` mode allows the `request_user_input` tool

### 3.2 CollaborationModeMask

**Location:** `protocol/src/config_types.rs`

```rust
pub struct CollaborationModeMask {
    pub name: String,
    pub mode: Option<ModeKind>,
    pub model: Option<String>,
    pub reasoning_effort: Option<Option<ReasoningEffort>>,
    pub developer_instructions: Option<Option<String>>,
}
```

### 3.3 Plan Mode Preset

**Location:** `tui_app_server/src/model_catalog.rs`

```rust
fn plan_preset() -> CollaborationModeMask {
    CollaborationModeMask {
        name: ModeKind::Plan.display_name().to_string(),
        mode: Some(ModeKind::Plan),
        model: None,
        reasoning_effort: Some(Some(ReasoningEffort::Medium)),
        developer_instructions: Some(Some(COLLABORATION_MODE_PLAN.to_string())),
    }
}
```

**Default Configuration:**
- **Reasoning Effort**: Medium (default override)
- **Developer Instructions**: Full plan mode system prompt from template

---

## 4. System Prompts

### 4.1 Plan Mode System Prompt

**Location:** `core/templates/collaboration_mode/plan.md`

The plan mode prompt defines a **3-phase conversational planning process**:

#### Phase 1: Ground in the Environment
- Explore repository before asking questions
- Run targeted searches and inspections
- Resolve ambiguities through discovery, not questions
- Silent exploration between turns is encouraged

#### Phase 2: Intent Chat
- Clarify goal + success criteria
- Identify audience, in/out of scope, constraints
- Understand current state and preferences
- Ask until intent is unambiguous

#### Phase 3: Implementation Chat
- Define approach and interfaces (APIs/schemas/I/O)
- Document data flow and edge cases
- Specify testing + acceptance criteria
- Plan rollout/monitoring and migrations

### 4.2 Key Mode Rules

**Execution vs. Mutation in Plan Mode:**

| Allowed (Non-Mutating) | Not Allowed (Mutating) |
|------------------------|------------------------|
| Reading/searching files | Editing/writing files |
| Static analysis | Running formatters/linters |
| Dry-run commands | Applying patches/migrations |
| Tests/builds (cache-only) | Codegen that updates tracked files |

**Plan Mode vs `update_plan` Tool:**
- Plan Mode is a **collaboration mode** for conversational planning
- `update_plan` is a **checklist/TODO tool** for tracking progress
- `update_plan` is **explicitly disabled** in Plan Mode (returns error)
- Do not confuse the two concepts

### 4.3 Final Plan Format

Plans must be wrapped in `<proposed_plan>` XML blocks:

```xml
<proposed_plan>
# Plan Title

## Summary
Brief description of what will be built

## Key Changes
- Grouped implementation bullets by subsystem
- Mention files only when needed for disambiguation

## Test Plan
- Test cases and scenarios

## Assumptions
- Explicit assumptions and defaults chosen
</proposed_plan>
```

**Formatting Rules:**
1. Opening tag on its own line
2. Content starts on next line
3. Closing tag on its own line
4. Use Markdown inside the block
5. Keep tags exactly as `<proposed_plan>` (do not translate)

---

## 5. Streaming Parser Architecture

### 5.1 AssistantTextStreamParser

**Location:** `utils/stream-parser/src/assistant_text.rs`

Parses assistant text streaming in one pass:

```rust
pub struct AssistantTextStreamParser {
    plan_mode: bool,
    citations: CitationStreamParser,
    plan: ProposedPlanParser,
}

impl AssistantTextStreamParser {
    pub fn new(plan_mode: bool) -> Self {
        Self {
            plan_mode,
            ..Self::default()
        }
    }

    pub fn push_str(&mut self, chunk: &str) -> AssistantTextChunk {
        let citation_chunk = self.citations.push_str(chunk);
        let mut out = self.parse_visible_text(citation_chunk.visible_text);
        out.citations = citation_chunk.extracted;
        out
    }
}
```

**Output Structure:**
```rust
pub struct AssistantTextChunk {
    pub visible_text: String,      // Text with tags stripped
    pub citations: Vec<String>,    // Extracted citation payloads
    pub plan_segments: Vec<ProposedPlanSegment>,
}
```

### 5.2 ProposedPlanParser

**Location:** `utils/stream-parser/src/proposed_plan.rs`

Streams `<proposed_plan>` block segments:

```rust
pub enum ProposedPlanSegment {
    Normal(String),
    ProposedPlanStart,
    ProposedPlanDelta(String),
    ProposedPlanEnd,
}
```

**Parsing Logic:**
```rust
const OPEN_TAG: &str = "<proposed_plan>";
const CLOSE_TAG: &str = "</proposed_plan>";

impl StreamTextParser for ProposedPlanParser {
    type Extracted = ProposedPlanSegment;

    fn push_str(&mut self, chunk: &str) -> StreamTextChunk<Self::Extracted> {
        map_segments(self.parser.parse(chunk))
    }
}
```

**Utility Functions:**
```rust
// Strip plan blocks from visible text
pub fn strip_proposed_plan_blocks(text: &str) -> String;

// Extract plan text for rendering
pub fn extract_proposed_plan_text(text: &str) -> Option<String>;
```

### 5.3 Stream Controller Integration

**Location:** `tui/src/streaming/controller.rs`

The stream controller renders plan segments with special styling:

```rust
fn handle_plan_segments(
    &mut self,
    segments: Vec<ProposedPlanSegment>,
) -> Option<Box<dyn HistoryCell>> {
    let mut plan_lines: Vec<Line<'static>> = Vec::new();
    
    // Add padding and format segments
    let plan_style = proposed_plan_style();
    let plan_lines = prefix_lines(plan_lines, "  ".into(), "  ".into())
        .into_iter()
        .map(|line| line.style(plan_style))
        .collect::<Vec<_>>();
    
    Some(Box::new(history_cell::new_proposed_plan_stream(
        plan_lines,
        is_stream_continuation,
    )))
}
```

---

## 6. User Interface Components

### 6.1 Plan Mode Indicators

**Location:** `tui/src/chatwidget.rs`

```rust
const PLAN_IMPLEMENTATION_TITLE: &str = "Implement this plan?";
const PLAN_IMPLEMENTATION_YES: &str = "Yes, implement this plan";
const PLAN_IMPLEMENTATION_NO: &str = "No, stay in Plan mode";
const PLAN_IMPLEMENTATION_CODING_MESSAGE: &str = "Implement the plan.";
```

### 6.2 Implementation Prompt

After a `<proposed_plan>` is output, the TUI shows a prompt:

**Location:** `tui/src/chatwidget.rs:1850-1896`

```rust
fn open_plan_implementation_prompt(&mut self) {
    let default_mask = collaboration_modes::default_mode_mask(self.models_manager.as_ref());
    
    let items = vec![
        SelectionItem {
            name: PLAN_IMPLEMENTATION_YES.to_string(),
            description: Some("Switch to Default and start coding.".to_string()),
            actions: vec![Box::new(move |tx| {
                tx.send(AppEvent::SubmitUserMessageWithMode {
                    text: PLAN_IMPLEMENTATION_CODING_MESSAGE.to_string(),
                    collaboration_mode: default_mask.clone(),
                });
            })],
            dismiss_on_select: true,
        },
        SelectionItem {
            name: PLAN_IMPLEMENTATION_NO.to_string(),
            description: Some("Continue planning with the model.".to_string()),
            actions: Vec::new(),
            dismiss_on_select: true,
        },
    ];

    self.bottom_pane.show_selection_view(SelectionViewParams {
        title: Some(PLAN_IMPLEMENTATION_TITLE.to_string()),
        items,
        ..Default::default()
    });
    
    self.notify(Notification::PlanModePrompt {
        title: PLAN_IMPLEMENTATION_TITLE.to_string(),
    });
}
```

### 6.3 Reasoning Scope Prompt

When changing reasoning effort in Plan mode, users are prompted for scope:

**Location:** `tui/src/chatwidget.rs:6685-6770`

```rust
pub(crate) fn open_plan_reasoning_scope_prompt(
    &mut self,
    model: &str,
    effort: Option<ReasoningEffortConfig>,
) {
    let items = vec![
        SelectionItem {
            name: PLAN_MODE_REASONING_SCOPE_PLAN_ONLY.to_string(),
            description: Some("Apply to Plan mode override".to_string()),
            actions: vec![Box::new(move |tx| {
                tx.send(AppEvent::UpdatePlanModeReasoningEffort(effort));
                tx.send(AppEvent::PersistPlanModeReasoningEffort(effort));
            })],
        },
        SelectionItem {
            name: PLAN_MODE_REASONING_SCOPE_ALL_MODES.to_string(),
            description: Some("Apply to global default and Plan mode override".to_string()),
            actions: vec![Box::new(move |tx| {
                tx.send(AppEvent::UpdateAllModesReasoning(effort));
                tx.send(AppEvent::PersistAllModesReasoning(effort));
            })],
        },
    ];
    
    self.bottom_pane.show_selection_view(SelectionViewParams {
        title: Some(PLAN_MODE_REASONING_SCOPE_TITLE.to_string()),
        items,
        ..Default::default()
    });
}
```

### 6.4 Plan Mode Styling

**Location:** `tui/src/style.rs`

```rust
pub fn proposed_plan_style() -> Style {
    proposed_plan_style_for(default_bg())
}

pub fn proposed_plan_style_for(terminal_bg: Option<(u8, u8, u8)>) -> Style {
    // Returns styled appearance for plan blocks
    // Typically uses distinct colors to differentiate from normal text
}
```

---

## 7. Slash Commands

### 7.1 `/plan` Command

**Location:** `tui/src/slash_command.rs`

```rust
impl SlashCommand {
    pub fn description(&self) -> &'static str {
        match self {
            SlashCommand::Plan => "switch to Plan mode",
            // ...
        }
    }
}
```

**Behavior:**
- Switches collaboration mode to Plan
- Applies Plan mode system prompt
- Sets reasoning effort to Medium (default)

### 7.2 Command Visibility

**Location:** `tui/src/bottom_pane/command_popup.rs`

The `/plan` command is:
- **Hidden** when collaboration modes are disabled
- **Visible** when collaboration modes are enabled

---

## 8. Tool System Integration

### 8.1 Plan Tool Handler

**Location:** `core/src/tools/handlers/plan.rs`

The `update_plan` tool is explicitly disabled in Plan mode:

```rust
pub(crate) async fn handle_update_plan(
    session: &Session,
    turn_context: &TurnContext,
    arguments: String,
    _call_id: String,
) -> Result<String, FunctionCallError> {
    if turn_context.collaboration_mode.mode == ModeKind::Plan {
        return Err(FunctionCallError::RespondToModel(
            "update_plan is a TODO/checklist tool and is not allowed in Plan mode".to_string(),
        ));
    }
    
    let args = parse_update_plan_arguments(&arguments)?;
    session.send_event(turn_context, EventMsg::PlanUpdate(args)).await;
    Ok("Plan updated".to_string())
}
```

### 8.2 Tool Specifications

**Location:** `protocol/src/plan_tool.rs`

```rust
#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema, TS)]
pub struct PlanItemArg {
    pub step: String,
    pub status: StepStatus,  // Pending, InProgress, Completed
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema, TS)]
pub struct UpdatePlanArgs {
    pub explanation: Option<String>,
    pub plan: Vec<PlanItemArg>,
}
```

**Tool Definition:**
```rust
pub static PLAN_TOOL: LazyLock<ToolSpec> = LazyLock::new(|| {
    ToolSpec::Function(ResponsesApiTool {
        name: "update_plan".to_string(),
        description: r#"Updates the task plan.
Provide an optional explanation and a list of plan items, each with a step and status.
At most one step can be in_progress at a time.
"#.to_string(),
        strict: false,
        parameters: JsonSchema::Object { /* ... */ },
    })
});
```

---

## 9. Collaboration Mode Management

### 9.1 Mode Mask Functions

**Location:** `tui/src/collaboration_modes.rs`

```rust
/// Get the Plan mode mask
pub(crate) fn plan_mask(models_manager: &ModelsManager) -> Option<CollaborationModeMask> {
    mask_for_kind(models_manager, ModeKind::Plan)
}

/// Get the Default mode mask
pub(crate) fn default_mode_mask(models_manager: &ModelsManager) -> Option<CollaborationModeMask> {
    mask_for_kind(models_manager, ModeKind::Default)
}

/// Cycle to the next collaboration mode preset
pub(crate) fn next_mask(
    models_manager: &ModelsManager,
    current: Option<&CollaborationModeMask>,
) -> Option<CollaborationModeMask> {
    let presets = filtered_presets(models_manager);
    if presets.is_empty() {
        return None;
    }
    let current_kind = current.and_then(|mask| mask.mode);
    let next_index = presets
        .iter()
        .position(|mask| mask.mode == current_kind)
        .map_or(0, |idx| (idx + 1) % presets.len());
    presets.get(next_index).cloned()
}
```

### 9.2 Visible Modes Configuration

**Location:** `protocol/src/config_types.rs`

```rust
pub const TUI_VISIBLE_COLLABORATION_MODES: [ModeKind; 2] = [
    ModeKind::Default,
    ModeKind::Plan,
];
```

Only these two modes are shown in the TUI mode switcher.

---

## 10. App Events

### 10.1 Plan Mode Events

**Location:** `tui/src/app_event.rs`

```rust
pub enum AppEvent {
    /// Open the Plan-mode reasoning scope prompt
    OpenPlanReasoningScopePrompt {
        model: String,
        effort: Option<ReasoningEffort>,
    },
    
    /// Update the Plan-mode-specific reasoning effort in memory
    UpdatePlanModeReasoningEffort(Option<ReasoningEffort>),
    
    /// Persist the Plan-mode-specific reasoning effort
    PersistPlanModeReasoningEffort(Option<ReasoningEffort>),
    
    /// Submit user message with specific collaboration mode
    SubmitUserMessageWithMode {
        text: String,
        collaboration_mode: CollaborationModeMask,
    },
}
```

### 10.2 Notification Events

**Location:** `tui/src/chatwidget.rs`

```rust
pub enum Notification {
    PlanModePrompt {
        title: String,
    },
    // ...
}

impl Notification {
    fn type_name(&self) -> &str {
        match self {
            Notification::PlanModePrompt { .. } => "plan-mode-prompt",
            // ...
        }
    }
}
```

---

## 11. Testing

### 11.1 Plan Mode Tests

**Location:** `tui/src/chatwidget/tests.rs`

Key test scenarios:

```rust
/// Plan implementation popup snapshot
#[tokio::test]
async fn plan_implementation_popup_snapshot() {
    let mut chat = create_test_chat();
    chat.open_plan_implementation_prompt();
    let popup = chat.bottom_pane.snapshot();
    assert_snapshot!("plan_implementation_popup", popup);
}

/// Plan slash command switches to Plan mode
#[tokio::test]
async fn plan_slash_command_switches_to_plan_mode() {
    // Test /plan command behavior
}

/// Reasoning selection in Plan mode opens scope prompt
#[tokio::test]
async fn reasoning_selection_in_plan_mode_opens_scope_prompt_event() {
    // Test reasoning effort change prompts
}

/// Plan mode reasoning override is marked current
#[tokio::test]
async fn plan_mode_reasoning_override_is_marked_current_in_reasoning_popup() {
    // Test UI state management
}
```

### 11.2 App Server Tests

**Location:** `app-server/tests/suite/v2/plan_item.rs`

Tests for `<proposed_plan>` block handling:

```rust
#[tokio::test]
async fn plan_mode_uses_proposed_plan_block_for_plan_item() -> Result<()> {
    let plan_block = "<proposed_plan>\n# Final plan\n- first\n- second\n</proposed_plan>\n";
    
    // Test that plan blocks are extracted and emitted as Plan items
    // Test that agent messages are still emitted alongside plan items
}

#[tokio::test]
async fn plan_mode_without_proposed_plan_does_not_emit_plan_item() -> Result<()> {
    // Test that normal messages don't create plan items
}
```

### 11.3 Stream Parser Tests

**Location:** `utils/stream-parser/src/proposed_plan.rs`

```rust
#[test]
fn streams_proposed_plan_segments_and_visible_text() {
    let mut parser = ProposedPlanParser::new();
    let out = collect_chunks(
        &mut parser,
        &[
            "Intro text\n<prop",
            "osed_plan>\n- step 1\n",
            "</proposed_plan>\nOutro",
        ],
    );

    assert_eq!(out.visible_text, "Intro text\nOutro");
    assert_eq!(
        out.extracted,
        vec![
            ProposedPlanSegment::Normal("Intro text\n".to_string()),
            ProposedPlanSegment::ProposedPlanStart,
            ProposedPlanSegment::ProposedPlanDelta("- step 1\n".to_string()),
            ProposedPlanSegment::ProposedPlanEnd,
            ProposedPlanSegment::Normal("Outro".to_string()),
        ]
    );
}

#[test]
fn strips_proposed_plan_blocks_from_text() {
    let text = "before\n<proposed_plan>\n- step\n</proposed_plan>\nafter";
    assert_eq!(strip_proposed_plan_blocks(text), "before\nafter");
}
```

---

## 12. Key File Locations

| Component | File Path | Description |
|-----------|-----------|-------------|
| ModeKind enum | `protocol/src/config_types.rs` | Mode type definitions |
| Plan mode template | `core/templates/collaboration_mode/plan.md` | System prompt |
| Default mode template | `core/templates/collaboration_mode/default.md` | Default system prompt |
| Plan tool handler | `core/src/tools/handlers/plan.rs` | `update_plan` tool logic |
| Plan tool types | `protocol/src/plan_tool.rs` | Tool argument types |
| Proposed plan parser | `utils/stream-parser/src/proposed_plan.rs` | XML block parsing |
| Assistant text parser | `utils/stream-parser/src/assistant_text.rs` | Combined streaming parser |
| Stream controller | `tui/src/streaming/controller.rs` | Plan segment rendering |
| Collaboration modes | `tui/src/collaboration_modes.rs` | Mode mask utilities |
| Model catalog | `tui_app_server/src/model_catalog.rs` | Mode preset definitions |
| Chatwidget | `tui/src/chatwidget.rs` | UI prompts and notifications |
| App events | `tui/src/app_event.rs` | Event type definitions |

---

## 13. Design Principles

### 13.1 Decision Completeness

Plans must be **decision complete**—the implementer should not need to make any decisions. This means:
- All interfaces/APIs are specified
- Edge cases and failure modes are documented
- Testing criteria are explicit
- Assumptions are recorded

### 13.2 Exploration First

**Golden Rule:** Before asking any question, perform at least one targeted non-mutating exploration pass.

Exceptions:
- Obvious ambiguities or contradictions in the prompt itself
- Questions that cannot be answered through exploration

### 13.3 Question Quality

Questions must:
- Materially change the spec/plan, OR
- Confirm/lock an important assumption, OR
- Choose between meaningful tradeoffs

**Preferred Method:** Use `request_user_input` tool with multiple-choice options.

### 13.4 Plan Compression

Final plans should be:
- **Concise by default**: 3-5 short sections
- **Grouped by subsystem**: Not file-by-file inventories
- **Behavior-focused**: Not symbol-by-symbol details
- **Assumption-explicit**: Record defaults chosen for unanswered questions

---

## 14. Common Patterns

### 14.1 Mode Switching Flow

```
User types "/plan" or clicks mode switch
    │
    ▼
ChatWidget switches to Plan mode mask
    │
    ▼
System prompt changes to plan.md template
    │
    ▼
Reasoning effort set to Medium (default)
    │
    ▼
Model receives updated instructions
```

### 14.2 Plan Output Flow

```
Model generates response with <proposed_plan> block
    │
    ▼
AssistantTextStreamParser parses chunks
    │
    ├──► Visible text: Plan blocks stripped
    └──► Plan segments: Extracted separately
    │
    ▼
StreamController renders plan with special style
    │
    ▼
HistoryCell created for plan display
    │
    ▼
Implementation prompt shown to user
```

### 14.3 Implementation Handoff

```
User selects "Yes, implement this plan"
    │
    ▼
AppEvent::SubmitUserMessageWithMode emitted
    │
    ├──► Text: "Implement the plan."
    └──► Mode: Default mode mask
    │
    ▼
New turn starts in Default mode
    │
    ▼
Model receives implementation instructions
```

---

## 15. Error Handling

### 15.1 Tool Restrictions

Attempting to use `update_plan` in Plan mode:
```rust
Err(FunctionCallError::RespondToModel(
    "update_plan is a TODO/checklist tool and is not allowed in Plan mode".to_string(),
))
```

### 15.2 Unterminated Plan Blocks

The parser automatically closes unterminated `<proposed_plan>` blocks on `finish()`:
```rust
#[test]
fn closes_unterminated_plan_block_on_finish() {
    let mut parser = ProposedPlanParser::new();
    let out = collect_chunks(&mut parser, &["<proposed_plan>\n- step 1\n"]);

    assert_eq!(out.extracted, vec![
        ProposedPlanSegment::ProposedPlanStart,
        ProposedPlanSegment::ProposedPlanDelta("- step 1\n".to_string()),
        ProposedPlanSegment::ProposedPlanEnd,  // Auto-closed
    ]);
}
```

---

## 16. Configuration

### 16.1 Plan Mode Defaults

| Setting | Value |
|---------|-------|
| Reasoning Effort | Medium |
| Request User Input | Available |
| Sandbox Mode | Inherited from config |
| Model | User-selected or default |

### 16.2 Persistence

Plan mode reasoning effort can be persisted per-profile:
```rust
AppEvent::PersistPlanModeReasoningEffort(effort)
```

Stored in config under:
```toml
[profiles.<name>]
plan_mode_reasoning_effort = "medium"
```

---

**Document Generated:** Based on analysis of codex-rs codebase

**Source of Truth:** All information derived directly from source code analysis.
