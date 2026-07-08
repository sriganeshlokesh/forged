# Interactive Suggestions & Guided Resume Revision

**Status:** proposed · **Date:** 2026-07-07 · **Scope:** forged (this repo) + drafted (`../drafted`)

Turn evaluation suggestions from display-only advice into one-click, validated resume edits. The user selects a suggestion, optionally answers short questions (facts the LLM must never invent), the backend generates and validates a proposed change, and the frontend previews the diff before applying it locally.

> **Phase plans (implementation blueprints):**
> [Phase 1 — Actionable suggestions + apply loop](phase-1-apply-loop.md) ·
> [Phase 2 — User-provided facts](phase-2-user-facts.md) ·
> [Phase 3 — Breadth: new actions, batch, lift tracking](phase-3-breadth.md) ·
> [Phase 4 — Future roadmap](phase-4-future-roadmap.md)
> This file stays the summary; the phase docs are authoritative for implementation detail.

---

## Interaction model (the revise loop)

1. `/v1/evaluations` returns **actionable suggestions**: each carries a machine-readable action (type + target + optional input questions) alongside today's `text/section/dimension/estimated_lift`.
2. User picks a suggestion in drafted. If it declares `inputs`, the card expands into a small form.
3. Frontend calls `POST /v1/revisions` with the **target slice** (not the full resume), the suggestion echoed back verbatim, and the user's answers.
4. forged validates the request, runs a tightly scoped LLM rewrite of the target field only, then runs **deterministic guardrails** (anti-fabrication, HTML allowlist, shape checks).
5. Response is a **proposed change** (`before`/`after` + rationale) — never a mutated resume.
6. drafted shows the diff; on accept it writes the change into its own state, offers undo, and nudges a re-score so the user sees realized vs. estimated lift.

## Locked design decisions

| Decision | Rationale |
|---|---|
| Backend returns **proposed patches**, never an updated resume | Small payloads, natural diff UX, user is always the final gate |
| Backend stays **stateless — no DB** | Client state mutates after every apply, so any server copy is instantly stale; storing resumes creates PII custody obligations; "we never store your resume" is a product feature |
| `/v1/revisions` takes a **target slice**, not the full resume | The rewrite only needs target content + suggestion + answers + JD (~3–6 KB). Guardrails need only `target.content ∪ answers` |
| **Stable item IDs are client-generated** (`crypto.randomUUID()`), backend round-trips them as opaque strings | Fixes index-staleness on reorder/delete, fixes React `key={i}` fragility; domain stays pure |
| Guardrails are **deterministic domain code**, not a second LLM call | Trust guarantee must not depend on model behavior: facts only enter the resume from the user |
| Suggestions **degrade gracefully** — `action` is optional on the wire | Old clients, stub evaluator, and unparseable LLM targets all fall back to today's display-only + jump-to-section |

## Wire contracts

### Extended suggestion (in `EvaluationResponse`)

```json
{
  "text": "Quantify the impact of the payments migration bullet",
  "section": "experience",
  "dimension": "impact_evidence",
  "estimated_lift": 6,
  "action": {
    "type": "rewrite_field",
    "target": { "section": "experience", "item_id": "a1b2…", "field": "bullets" },
    "inputs": [
      { "key": "metric", "label": "Roughly how many transactions/users did this affect?", "required": true },
      { "key": "timeframe", "label": "Over what period?", "required": false }
    ]
  }
}
```

`action` is omitted when the model can't produce a valid target (parser drops invalid ones). `inputs` arrives in Phase 2.

### `POST /v1/revisions` — request (target slice)

```json
{
  "job_description": "…",
  "suggestion": { "…the suggestion object echoed back verbatim…": "" },
  "answers": { "metric": "≈2M transactions/day", "timeframe": "6 months" },
  "target": {
    "field": "bullets",
    "content": "<ul><li>Migrated payments service…</li></ul>",
    "context": { "company": "Stripe", "role": "Software Engineer" }
  }
}
```

`answers` is Phase 2; empty object until then. `target.context` is optional flavor for the rewrite prompt.

### `POST /v1/revisions` — response

```json
{
  "status": "ok",
  "changes": [{
    "target": { "section": "experience", "item_id": "a1b2…", "field": "bullets" },
    "before": "<ul><li>Migrated payments service…</li></ul>",
    "after":  "<ul><li>Migrated payments service handling ≈2M transactions/day…</li></ul>",
    "rationale": "Added the throughput figure you provided and tightened the verb."
  }],
  "warnings": []
}
```

### Validation pipeline (order matters)

1. **Transport** — 1 MB body cap, JSON decode → 400/10001.
2. **Structural** (`Input.Validate()`) — action type known, `target.field` matches `suggestion.action.target.field`, non-empty content, required answers present, length caps → 400/10002.
3. **Constrained generation** — `pkg/atseval.Revise()` sends only target content + suggestion + answers + JD context; demands JSON back (same `json_schema` → `json_object` fallback as `client.go`).
4. **Deterministic guardrails** (`domain/service`) —
   - *Anti-fabrication:* every number/percentage token in `after` must appear in `before` ∪ `answers`. The model may rephrase; facts come only from the user.
   - *HTML allowlist:* only tags Tiptap produces (`p`, `ul`, `li`, `strong`, `em`, `br`).
   - *Shape:* bullets stay `<ul>` with ≥1 `<li>`; field length caps; non-empty output.
5. **On guardrail failure** — one retry with the violation fed back to the model, then 503/30003 (`ErrRevisionFailed`).

---

## Phase 1 — Actionable suggestions + the apply loop

Goal: "one-click fixes" for `rewrite_field` (summary, `experience[].bullets`, `projects[].description`). No user-input forms yet; anti-fabrication uses `before` only.

### forged

- [ ] `domain/model/revision.go` — `SuggestionAction`, `RevisionTarget`, `Revision`, `Change`; sentinels `ErrUnknownAction`, `ErrTargetMismatch`, `ErrRevisionFailed`, `ErrUnsafeContent`. Extend `Suggestion` with optional `Action`.
- [ ] `domain/service/` — guardrail functions (number extraction/containment, HTML allowlist, shape checks). Stdlib only, table-driven tests.
- [ ] `pkg/atseval` (stays stdlib-only) —
  - `renderResume` gains item markers (`### [exp:<id>] Role, Company`); IDs capped (≤64 chars) at DTO→model mapping to avoid prompt bloat.
  - Evaluation JSON schema + prompt extended with `action` (type, target ids); parser validates returned `item_id`s against the submitted resume and **drops invalid actions** (suggestion survives as display-only).
  - New `Revise(ctx, RevisionRequest) (*RevisionResult, error)` with its own system prompt + schema, reusing `client.go` machinery.
- [ ] `application/revision/usecase.go` — implements `core.UseCase`; declares consumer interface `ResumeReviser` in the same file; `Input.Validate()` covers step 2 of the pipeline.
- [ ] `adapter/llm/atseval` — extend adapter with `Revise` mapping domain ↔ pkg types; wrap failures as `ErrRevisionFailed`.
- [ ] `adapter/evaluator/stub` — stub reviser when `LLM_API_KEY` is empty: deterministic transform (e.g. appends a marker bullet) so drafted development never blocks on a key.
- [ ] `api/dto/revision.go` — request/response DTOs (snake_case), mapping to/from domain.
- [ ] `api/http/handle/revision.go` — handler with consumer interface `RevisionUseCase`; error mapping: 10001 decode, 10002 validation, 10003 rate limit, 30003 revision failed (503), 30001 fallback.
- [ ] `api/http/router.go` — `POST /v1/revisions` behind per-IP rate limiting.
- [ ] `api/error_code/` — add 30003 (`ErrRevision`).
- [ ] `config/config.go` — `RATE_LIMIT_REVISIONS_PER_IP_RPM` (default 20, 0 disables); document in README env table.
- [ ] Wiring: `adapter/dependency/providers.go` (stub vs real selection) + `wire.go` → `make wire`; register `ResumeReviser`/`RevisionUseCase` in `.mockery.yaml` → `make mocks`.
- [ ] Tests: handler via `httptest` + mockery mocks; use case table-driven; `pkg/atseval` `Revise` with fake `Options.HTTPClient`; guardrail table tests. `make all` green.

### drafted

- [ ] Stable IDs: add `id: string` to `Experience`, `Project`, `Education`, `SkillGroup` in `src/types.ts`; generate in `addItem` and `normalizeResumeData()` (localStorage migration); use as React keys; include as `id` in the evaluation request mapping (`src/evaluation.ts toEvaluationRequest`).
- [ ] `src/evaluation.ts` — extend `EvaluationSuggestion` with optional `action`; add `reviseResume()` client for `/v1/revisions`, reusing `EvaluationError` + `FRIENDLY_MESSAGES` (add 30003 copy).
- [ ] `SuggestedEditsCard` (`src/JobMatch.tsx`) — keep `+N pts` badge and jump-to-section; add **Apply** for suggestions with `action`; per-row spinner + error state.
- [ ] Diff preview modal — stacked before/after rendered as rich text + rationale; Accept / Discard.
- [ ] Apply on accept — resolve `item_id` → index, write via existing `setItemField`/`patch`; if the id no longer exists: "this part of your resume changed — re-run the match".
- [ ] Undo — in-memory snapshot stack (capped ~20) pushed before each apply; "Applied — Undo" toast.
- [ ] Applied-state marking (check/strike) persisted in `drafted:match:v1`; "Re-run match" nudge (existing `evalPrevScore` delta chip shows realized lift).

**Acceptance:** user applies a rewrite suggestion end-to-end against the stub with no LLM key; with a key, guardrails reject fabricated numbers; reordering entries between evaluate and apply never corrupts the wrong item.

## Phase 2 — User-provided facts

Goal: quantification suggestions become honest — the model asks, the user answers, guardrails enforce.

### forged

- [ ] `pkg/atseval` evaluation prompt/schema emit `inputs` (`key`, `label`, `required`) on actions that need facts; parser caps count (≤3) and label length.
- [ ] `/v1/revisions` DTO + `Input.Validate()` accept `answers`: required keys present, per-answer length caps, keys ⊆ declared inputs.
- [ ] Rewrite prompt includes answers as "facts provided by the candidate — use verbatim or not at all".
- [ ] Anti-fabrication set extends to `before ∪ answers`; unverifiable model additions stripped or retried; `warnings[]` populated when an answer was ignored.

### drafted

- [ ] Inline answer form rendered from `action.inputs` (required fields gate Apply); answers included in the revise request.
- [ ] UX copy making the trust story explicit ("we only add numbers you provide").

**Acceptance:** a "quantify this bullet" suggestion asks for a metric, and the returned `after` contains that metric and no other new numbers.

## Phase 3 — Breadth & polish

- [ ] New action types: `add_skill` (append to a skill group, dedup against existing `items`) and `add_bullet` (append `<li>` to target bullets) — action registry on the backend, per-type apply logic in drafted.
- [ ] Batch: "Apply all suggestions that need no input" — sequential revise calls, one combined preview, single undo snapshot.
- [ ] Auto re-score prompt after N applies; estimated vs. realized lift surfaced per suggestion.
- [ ] Suggestion quality iteration: tune prompt using which suggestions users apply vs. dismiss (client-side counts only, no server storage).
- [ ] Rate-limit and cost tuning once real usage patterns are visible.

---

## Future features unlocked by this foundation

Statelessness and the patch-based contract are deliberately future-proof: none of these change the `/v1/revisions` shape.

**No new infra required**
- **Rewrite variants** — `Revise` returns 2–3 alternatives; the diff modal becomes a picker.
- **Auto-fix mode** — apply every zero-input suggestion in sequence with one final review screen.
- **Evaluation caching** — hash(resume + JD) → short-TTL in-memory cache to absorb repeat runs cheaply.
- **Keyword coverage report** — deterministic JD-term extraction in `pkg/atseval` alongside the LLM rubric.
- **Cover letter / outreach generation** — same engine, same guardrail philosophy (facts from resume + user answers only).
- **Section-structure suggestions** — reorder/merge advice as display-only actions the FE executes locally.

**When a DB becomes justified** (accounts era — the honest reasons are product features, never payload size)
- Accounts + stored resumes → cross-device sync, version history with restore points.
- Shareable evaluation links (`/e/<id>`), application tracker across multiple JDs.
- Per-user rate limiting/quotas replacing per-IP; usage analytics to tune prompts and pricing.
- Server-side resolution of `target.content` becomes an *option* on the same contract.

**Platform direction**
- Multi-JD comparison ("which of these 5 roles am I closest to?").
- Interview-prep pack generated from gaps ("your weakest dimension is impact evidence — prepare these stories").
- Template/format-aware suggestions (LaTeX/Word export already exists in drafted).

---

## Architecture-law compliance map

| Piece | Layer | Path |
|---|---|---|
| Revision entity, action types, sentinels | domain | `domain/model/revision.go` |
| Guardrails (fact check, HTML allowlist) | domain | `domain/service/` |
| Revise use case + `ResumeReviser` interface | application | `application/revision/` |
| LLM revise engine (stdlib-only) | pkg | `pkg/atseval/` |
| Engine adapter + stub | adapter | `adapter/llm/atseval/`, `adapter/evaluator/stub/` |
| DTOs, handler, route | api | `api/dto/revision.go`, `api/http/handle/revision.go`, `api/http/router.go` |
| Binding + provider selection | composition root | `adapter/dependency/` |

Workflow reminders: every `wire.go` edit → `make wire`, commit both; every new consumer interface → `.mockery.yaml` → `make mocks`; `make all` before each commit; `/health` and PORT binding untouched.
