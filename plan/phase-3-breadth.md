# Phase 3 — Breadth: New Action Types, Batch Apply, Lift Tracking

**Status:** ready after Phase 2 · **Depends on:** [phase-1-apply-loop.md](phase-1-apply-loop.md), [phase-2-user-facts.md](phase-2-user-facts.md)
**Parent:** [interactive-suggestions.md](interactive-suggestions.md)

## Goal

Widen the apply loop from "rewrite one field" to the remaining high-value edits (**add a skill**, **add a bullet**), let users clear all easy suggestions in one pass (**batch apply**), and close the feedback loop by showing **estimated vs. realized lift** after re-scoring. Add dismiss + lightweight client-side metrics so future prompt tuning is informed by real accept/reject behavior.

**In scope:** `add_skill`, `add_bullet`, batch apply, dismiss, realized-lift UI, apply/dismiss counters, config knob review.
**Out of scope:** anything requiring server-side storage (Phase 4 territory).

---

## Part A — `add_skill`

### Wire contract

Suggestion:

```json
"action": {
  "type": "add_skill",
  "target": { "section": "skills", "item_id": "<skill-group-id>", "field": "items" }
}
```

Revision request: `target.field = "items"`, `target.content` = the group's current items serialized as a **JSON array string** (e.g. `"[\"Go\",\"PostgreSQL\"]"`) — unambiguous even when an item contains a comma. `target.context.name` = the group label (e.g. "Backend").

Revision response — `ChangeDTO` gains two optional fields:

```json
"changes": [{
  "op": "replace_items",
  "target": { "section": "skills", "item_id": "g1", "field": "items" },
  "before": "[\"Go\",\"PostgreSQL\"]",
  "items_after": ["Go", "PostgreSQL", "Kubernetes"],
  "rationale": "Added Kubernetes — required by the JD and evidenced by your deployment bullet."
}]
```

`op` defaults to `"replace"` (Phase 1/2 string semantics) when omitted; `"replace_items"` means the frontend replaces the group's `items` with `items_after` wholesale.

### Backend

- **Domain** (`domain/model/revision.go`): `ActionAddSkill ActionType = "add_skill"`; `Change` gains `Op string` and `ItemsAfter []string`; target-combo matrix gains `(skills, items, item_id required)`.
- **Guardrail** (`domain/service/revision.go`):

  ```go
  // ValidateSkillItems enforces, over (existingItems, newItems, jd, suggestionText, answers):
  //  - every ADDED item appears case-insensitively in jd ∪ suggestionText ∪ answer values
  //    (no invented skills — the JD is the evidence source here, not the resume)
  //  - existing items preserved verbatim and in order (additions appended)
  //  - ≤ 5 additions per revision; each item 1–40 chars, plain text (no HTML)
  //  - case-insensitive dedupe against existing items
  func ValidateSkillItems(existing, proposed []string, jd, suggestion string, answers map[string]string) error
  ```

- **pkg/atseval** (`revise.go`): when `ActionType == "add_skill"`, the response schema becomes `{"items_after": [string], "rationale": string}`; `RevisionResult` gains `ItemsAfter []string`. Revise prompt variant:

  > You add missing skills to one skill group. Only add skills that the job description explicitly names AND the candidate plausibly has given the group's context — when in doubt, add nothing. Never remove or rewrite existing items. Return JSON only: {"items_after": [...], "rationale": "..."}.

- **Application**: `Execute` branches on action type — skill path runs `ValidateSkillItems` (with the same one-retry-with-feedback loop) instead of the HTML/number guardrails; output `Change{Op: "replace_items", ItemsAfter: …}`.
- **Evaluation side**: schema `enum` for `action.type` grows to `["rewrite_field","add_skill","add_bullet"]`; renderResume already emits `[skill:<id>] Label` markers from Phase 1; parser matrix updated.

### Frontend

- `resolveTarget` handles `skills`: `content = JSON.stringify(group.items)`, `context = { name: group.label }`.
- `applyChange` handles `op === "replace_items"`: `setItemField('skillGroups', i, 'items', change.items_after)` (matches `SkillGroup.items: string[]`).
- Diff modal renders item changes as chips — unchanged items muted, additions highlighted — instead of HTML panels (branch on `op`).

---

## Part B — `add_bullet`

### Wire contract

```json
"action": {
  "type": "add_bullet",
  "target": { "section": "experience", "item_id": "a1b2c3", "field": "bullets" },
  "inputs": [{ "key": "achievement", "label": "What did you do here that isn't listed? Include a number if you can.", "required": true }]
}
```

`add_bullet` will almost always carry a required input — a new bullet made of nothing but existing text is what `rewrite_field` is for. Response is a normal `op: "replace"` change on the `bullets` field (full `<ul>` returned).

### Backend

- **Domain**: `ActionAddBullet ActionType = "add_bullet"`.
- **Guardrail** (`domain/service/revision.go`):

  ```go
  // ValidateBulletAddition enforces:
  //  - every <li> from `before` appears verbatim and in order in `after`
  //  - exactly ONE new <li>, appended last
  //  - the new bullet passes CheckNoNewNumbers against (before ∪ answers)
  //  - sanitization + shape + length caps (reuse Phase 1 checks)
  func ValidateBulletAddition(before, after string, extra map[string]struct{}) error
  ```

  "Verbatim" is compared on sanitized, whitespace-normalized `<li>` inner HTML, so incidental attribute stripping doesn't false-positive.

- **pkg/atseval**: prompt variant — "Append exactly one new bullet built ONLY from the candidate-provided facts and existing context; do not alter existing bullets." Same `{"after","rationale"}` schema as `rewrite_field`.
- **Application**: branches to `ValidateBulletAddition`; everything else (retry, warnings) identical.

### Frontend

Nothing new: it's a `replace` on `bulletsText`, so Phase 1/2 resolve/apply/diff/undo paths already handle it. The diff naturally shows one added `<li>`.

---

## Part C — Batch apply ("Fix the easy ones")

Frontend-only orchestration; the backend contract is untouched.

- Button on `SuggestedEditsCard`: **"Apply all N zero-input suggestions"** — eligible = has `action`, `inputs` empty/absent, target resolves, not applied/dismissed.
- Execution: **sequential** `reviseResume` calls (one in flight, ~300 ms spacing — the default 20 rpm revisions limit comfortably fits the schema's 3–6 suggestions; never parallelize against a per-IP limiter).
- Progress UI: the modal becomes a checklist — each row `pending → previewing → ok / failed`, showing the before/after for each completed item with a per-row checkbox (default checked).
- Confirmation: one **"Apply M selected changes"** button; a **single undo snapshot** taken before the batch lands (one Undo reverts the whole batch).
- Failure handling: a 429 pauses the queue with a visible countdown then resumes; 5xx marks that row failed and continues; nothing is applied until final confirmation.

## Part D — Dismiss + estimated vs. realized lift

- **Dismiss:** per-row ✕. Dismissed keys persist alongside applied keys in `drafted:match:v1`; both reset when a new evaluation lands. "Show dismissed (n)" link restores visibility. `pointsLeft` excludes applied **and** dismissed rows.
- **Realized lift:** when applies happen, store `pendingLiftEstimate = Σ estimated_lift` of applied suggestions (in the match report blob). On the next completed `runMatch()`, compute `realized = newScore − evalPrevScore` and render a chip next to the score delta: **"est +9 → got +6"**. Clear `pendingLiftEstimate` after rendering once. Helper + tests in `src/matchScore.ts`.
- **Auto re-score nudge:** after a batch completes, or after 2+ single applies, elevate the Phase 1 banner to a primary-styled prompt. Never auto-run the evaluation — it costs a rate-limited LLM call; the user stays in control.

## Part E — Client-side apply/dismiss counters

`localStorage` key `drafted:suggestmetrics:v1`:

```json
{ "applied": {"rewrite_field": 12, "add_skill": 4}, "dismissed": {"rewrite_field": 3}, "warnings_seen": 2 }
```

Incremented in the apply/dismiss handlers; no UI, no server transmission — this is groundwork for prompt tuning ("are `add_skill` suggestions being dismissed 80% of the time?"). Reading it happens manually in devtools until Phase 4 analytics exist.

## Part F — Knob review (backend, small)

- Revisit `RATE_LIMIT_REVISIONS_PER_IP_RPM` default (20) against real batch usage; bump only with evidence.
- Optional: `LLM_REVISE_MODEL` env (defaults to `LLM_MODEL`) — single-field rewrites are a natural fit for a smaller/faster model than the evaluation rubric. Plumb through `config` → `ProvideReviser` only; document in README. Skip if Groq latency is already fine.

---

## Commit sequence

Backend:
1. `feat(domain): add_skill and add_bullet actions with dedicated guardrails` (+ tests)
2. `feat(atseval): per-action revise prompts/schemas; action enum in evaluation` (+ tests)
3. `feat(application/api): op + items_after on changes; action-type branching` (+ tests, mocks, wire if interfaces moved)
4. `feat(config): optional LLM_REVISE_MODEL` (if taken)

Frontend:
1. `feat: add_skill apply path with chip diff`
2. `feat: batch apply with per-change confirmation and single undo`
3. `feat: dismiss, realized-lift chip, apply metrics counters`

## Test matrix (definition of done)

| # | Scenario | Expected |
|---|---|---|
| 1 | `add_skill` proposes a skill named in the JD | Chip diff shows the addition; accept updates `skillGroups[i].items` |
| 2 | Model proposes a skill absent from JD/suggestion/answers | Guardrail retry → 503/30003 if it persists |
| 3 | `add_skill` returns existing items reordered/dropped | Guardrail rejects (existing preserved verbatim, in order) |
| 4 | `add_bullet` with answered input | Exactly one new `<li>` appended; existing bullets byte-identical after sanitization |
| 5 | Batch of 5 zero-input suggestions | Sequential calls; checklist preview; one confirm; **one** undo reverts all |
| 6 | 429 mid-batch | Queue pauses with countdown, resumes, completes |
| 7 | Dismiss two suggestions | Excluded from `pointsLeft`; restorable; reset on new evaluation |
| 8 | Apply +9 estimated, re-run scores +6 | "est +9 → got +6" chip renders once, then clears |
| 9 | Phases 1–2 flows | Regression-free (`op` omitted still means `replace`) |
