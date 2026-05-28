# Substrate Survey — Ground Truth from Real CC Transcripts

Surveyed 2026-05-28 against `~/.claude/projects/` before designing the eval harness.
Goal: replace the brief's idealized record tuple with what the transcripts actually contain.

## Inventory

- **530 JSONL transcripts**, 1.8 GB total, across ~60 project dirs (one dir per repo path).
- Go repos present and dense: plancheck, hindcast, buddy, defn, claude-mv, etc.
- A single plancheck session = 16,748 lines, **5,010 tool_use blocks**.
- Phase 1's "100 historical CC interactions" requirement is met many times over **without
  building LENS first** — the eval harness can replay against existing transcripts today.
  (`fd -e jsonl` returns 0 here — it respects an ignore rule; use `find`.)

## Transcript line schema

Each line is one JSON object keyed by `.type`:

| type | role | carries |
|------|------|---------|
| `assistant` | — | `.message.content[]` blocks: `tool_use`, `text`, `thinking` |
| `user` | user | a turn prompt, OR a tool result (when `.toolUseResult != null`) |
| `attachment` | — | injected context (hooks, file snapshots) |
| `permission-mode` / `mode` / `last-prompt` / `ai-title` / `file-history-snapshot` | — | session metadata, not records |

Common fields on every real entry: `uuid`, `parentUuid`, `sessionId`, `timestamp` (ISO),
`cwd`, `gitBranch`, `version`, `isSidechain`.

## The record tuple maps cleanly onto transcripts

Brief tuple `(input_context, tool_called, tool_args, tool_result, claude_followup, repo, timestamp)`:

| brief field | source |
|-------------|--------|
| `tool_called` + `tool_args` | `assistant.message.content[].tool_use {id, name, input}` |
| `tool_result` | `user` entry with `.toolUseResult`, paired by `tool_use_id` |
| `claude_followup` | next `assistant` entry down the `parentUuid` chain |
| `input_context` | walk `parentUuid` back to preceding user prompt / assistant text |
| `repo` | `cwd` + `gitBranch` (on every line) |
| `timestamp` | ISO on every line |

**Pairing key:** the result's `tool_result` content block carries `tool_use_id`, matching the
`tool_use.id`. Verified 1:1 (3653 uses / 3652 results — the off-by-one is the session's final
un-resulted call). `parentUuid`/`uuid` form the turn DAG; `sourceToolAssistantUUID` also links
the result entry back to its emitting assistant turn.

## CRITICAL design constraint (not in the brief): tool_result is heterogeneously typed

`toolUseResult` has a **different shape per tool**. There is no single fidelity-match function;
the eval's comparator must dispatch on tool name:

| tool | toolUseResult keys |
|------|--------------------|
| Bash | `stdout, stderr, interrupted, isImage, noOutputExpected` (+`backgroundTaskId` for bg) |
| Read | `file, type` |
| Edit | `filePath, oldString, newString, originalFile, structuredPatch, userModified, replaceAll` |
| Write | `content, filePath, originalFile, structuredPatch, type` |
| Grep | `content, filenames, mode, numFiles, numLines` |
| Agent | `agentId, agentType, content, prompt, status, totalTokens, totalToolUseCount, usage` |
| TaskUpdate | `statusChange, success, taskId, updatedFields, verificationNudgeNeeded` |
| (some) | bare `string` |

Implication for **EVAL (Phase 1)**: fidelity is per-tool-typed. Bash → compare stdout + exit
intent; Edit → compare `structuredPatch`/`newString`; Read → file content; Grep → match set.
The synthetic-regression rig must inject tool-appropriate subtle wrongness (e.g. an Edit hook
that drops one hunk, a Bash hook that flips an exit code) and confirm the typed comparator
catches each.

## Tool-call distribution (one plancheck session, representative)

```
2517 Bash   950 Read   925 Edit   261 Grep   156 Write
 48 Agent    46 TaskUpdate  24 TaskCreate  20 ToolSearch  10 check_plan
```

Bash ≈ 50% of all calls. The crystallizable mechanical residue almost certainly lives in the
**Bash / Read / Grep tail** — high-frequency, low-output-variance invocations. This is the
signal GATE (Phase 3) will cluster on; "low semantic variance" should be measured on
`claude_followup` (what Claude does next), not just on the result text.

## Consequences for build order

1. **Phase 1 no longer blocks on Phase 2.** Eval harness reads existing transcripts. Build and
   validate the harness against real history now.
2. **Fidelity metric is a per-tool dispatch table**, not one function. Design it that way from
   the start.
3. **Substrate extractor** (later) is a transcript walker: pair tool_use↔result by id, attach
   followup + context via parentUuid, stamp repo/time. The live LENS PostToolUse hook and the
   offline transcript walker should emit the **same** record schema so eval replay and live
   capture are interchangeable.
