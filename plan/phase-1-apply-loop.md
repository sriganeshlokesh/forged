# Phase 1 — Actionable Suggestions + the Apply Loop

**Status:** ready to implement · **Depends on:** nothing (first phase) · **Repos:** forged + drafted
**Parent:** [interactive-suggestions.md](interactive-suggestions.md)

## Goal

A user runs a match, clicks **Apply** on a suggestion, sees a before/after diff, accepts it, and their resume updates locally — with the backend guaranteeing the rewrite added no facts that weren't already in the original text.

**In scope:** one action type, `rewrite_field`, over three targets: `summary` (resume-level), `experience[].bullets`, `projects[].description`. Stable item IDs. Diff preview, apply, undo, applied-marking, re-score nudge. Stub reviser for keyless dev.

**Out of scope (later phases):** user-input questions/answers (Phase 2), `add_skill`/`add_bullet`/batch apply (Phase 3).

---

## Wire contract (Phase 1 subset)

### Suggestion gains an optional `action`

```json
{
  "text": "Tighten the payments-migration bullet around outcomes",
  "section": "experience",
  "dimension": "impact_evidence",
  "estimated_lift": 6,
  "action": {
    "type": "rewrite_field",
    "target": { "section": "experience", "item_id": "a1b2c3", "field": "bullets" }
  }
}
```

`action` is omitted whenever the model can't name a valid single target — the suggestion then renders exactly as today (display-only + jump-to-section). The stub evaluator emits no actions; nothing breaks.

Valid `(section, field)` combos in Phase 1 — reject/drop everything else:

| section | field | item_id |
|---|---|---|
| `summary` | `summary` | must be empty |
| `experience` | `bullets` | required |
| `projects` | `description` | required |

### `POST /v1/revisions`

Request (target slice — never the full resume):

```json
{
  "job_description": "…",
  "suggestion": { "…full suggestion object echoed verbatim…": "" },
  "target": {
    "field": "bullets",
    "content": "<ul><li>Migrated payments service…</li></ul>",
    "context": { "company": "Stripe", "role": "Software Engineer" }
  }
}
```

`target.context` is optional flavor: `{company, role}` for experience, `{name}` for projects, `{role: <targetRole>}` for summary. All fields optional strings.

Response:

```json
{
  "status": "ok",
  "changes": [{
    "target": { "section": "experience", "item_id": "a1b2c3", "field": "bullets" },
    "before": "<ul><li>Migrated payments service…</li></ul>",
    "after":  "<ul><li>Led migration of the payments service…</li></ul>",
    "rationale": "Tightened verbs and led with the outcome."
  }],
  "warnings": []
}
```

`changes[].target` is echoed from `suggestion.action.target` so the frontend can re-resolve on accept. `before` is the submitted content, unmodified.

Errors (existing envelope `{code, message}`):

| HTTP | code | when |
|---|---|---|
| 400 | 10001 | JSON decode failure |
| 400 | 10002 | validation: missing/unknown action, field mismatch, empty content/JD, oversize content |
| 429 | 10003 | rate limited |
| 503 | **30003** (new) | LLM revise failed, or output failed guardrails after one retry |
| 500 | 30001 | anything else |

---

## Backend (forged)

### 1. Domain: stable IDs on resume items

`domain/model/resume.go` — add `ID string` as the first field of `Experience`, `Project`, `Education`, `SkillGroup`. IDs are **opaque client-generated strings**; the domain never creates or interprets them.

ID hygiene rule (enforce during DTO→model mapping, `api/dto/evaluation.go ToModel()`): keep an ID only if it matches `^[A-Za-z0-9_-]{1,64}$`; otherwise blank it (item just loses actionability, nothing fails).

### 2. Domain: revision model — `domain/model/revision.go` (new)

```go
type ActionType string

const ActionRewriteField ActionType = "rewrite_field"

type RevisionTarget struct {
	Section string // "summary" | "experience" | "projects"
	ItemID  string // "" for summary
	Field   string // "summary" | "bullets" | "description"
}

type SuggestionAction struct {
	Type   ActionType
	Target RevisionTarget
	// Inputs added in Phase 2
}

type Change struct {
	Target    RevisionTarget
	Before    string
	After     string
	Rationale string
}

type Revision struct {
	Changes  []Change
	Warnings []string
}
```

Extend `Suggestion` (in `domain/model/evaluation.go`) with `Action *SuggestionAction` (nil = display-only).

Sentinels (same style as the existing ones, checked with `errors.Is`):

```go
var (
	ErrUnknownAction      = errors.New("unknown or missing suggestion action")
	ErrTargetMismatch     = errors.New("target does not match suggestion action")
	ErrEmptyTargetContent = errors.New("target content must not be empty")
	ErrRevisionFailed     = errors.New("revision could not be produced") // → 503/30003
)
```

Add a validation helper used by both the evaluation parser and the revision input:

```go
// ValidTargetCombo reports whether (section, field, itemID presence) is a
// permitted Phase-1 rewrite target.
func ValidTargetCombo(t RevisionTarget) bool
```

### 3. Domain: guardrails — `domain/service/revision.go` (new, stdlib only)

Three deterministic checks. All operate on text content (strip tags first with a small internal helper — do **not** import from `pkg/atseval`; domain imports nothing outside stdlib/domain).

```go
// NumericFacts extracts number-ish tokens: optional currency symbol,
// digits with commas/decimals, optional %/k/K/M/B/x suffix.
// Tokens are normalized (commas stripped, lowercased suffix).
// Regex to start from: [$€£]?\d[\d,]*(?:\.\d+)?\s?(?:%|k|K|M|B|x)?
func NumericFacts(s string) map[string]struct{}

// CheckNoNewNumbers returns ErrUnsafeContent-style detail if `after`
// contains a numeric fact absent from before ∪ extra.
// Phase 1 passes extra = nil; Phase 2 passes answer-derived facts.
func CheckNoNewNumbers(before, after string, extra map[string]struct{}) error

// SanitizeHTML keeps only <p> <ul> <ol> <li> <strong> <em> <br>,
// strips every attribute, drops all other tags (keeping inner text),
// removes <script>/<style> together with their content.
func SanitizeHTML(s string) string

// ValidateShape enforces per-field structure on the sanitized `after`:
//   bullets:              contains <ul> or <ol> with ≥1 <li>
//   summary/description:  non-empty text content
//   all fields:           len(after) ≤ 4000 bytes AND ≤ 3×len(before)+500
func ValidateShape(field, before, after string) error
```

Failures return descriptive errors (they get fed back to the model on retry), all wrapping a package-level `ErrUnsafe` so the use case can classify them.

Tests: table-driven — numbers with commas/percents/currency/suffixes; year added by model → rejected; same numbers reordered → pass; `<script>alert(1)</script>` stripped; `<li>` count preserved; oversize rejected; bullets returned as `<p>` → shape failure.

### 4. `pkg/atseval`: evaluation emits actions (stays stdlib-only)

**Resume IDs.** Add `ID string` to `pkg/atseval` `Experience`, `Project`, `Education`, `SkillGroup`.

**Markers in `renderResume`.** When an item's ID is non-empty, prefix its heading with a bracketed marker the model can cite:

```
### [exp:a1b2c3] Software Engineer — Stripe
### [prj:9f8e7d] drafted (resume builder)
```

No ID → heading rendered exactly as today (backward compatible with clients that don't send IDs yet).

**Schema + prompt.** Extend the suggestion object in the evaluation JSON schema with a nullable `action`:

```json
"action": {
  "type": ["object", "null"],
  "properties": {
    "type":   { "type": "string", "enum": ["rewrite_field"] },
    "target": {
      "type": "object",
      "properties": {
        "section": { "type": "string" },
        "item_id": { "type": "string" },
        "field":   { "type": "string" }
      },
      "required": ["section", "item_id", "field"],
      "additionalProperties": false
    }
  },
  "required": ["type", "target"],
  "additionalProperties": false
}
```

Prompt addition (system prompt, after the suggestion rules):

> When a suggestion targets one specific entry, include an `action` of type `rewrite_field` whose `target.item_id` is the bracketed id shown in the resume (e.g. `[exp:a1b2c3]` → `"a1b2c3"`). Valid targets: section `summary` with field `summary` and empty item_id; section `experience` with field `bullets`; section `projects` with field `description`. If a suggestion has no single target, set `action` to null.

**Parser hardening** (in the existing normalization pass, `atseval.go`): build the set of known IDs from the submitted resume; drop `action` (set nil, keep the suggestion) when — type unknown, combo invalid per the matrix, `item_id` unknown, or `item_id` non-empty for summary. Never fail the whole evaluation over a bad action.

Tests: fake `Options.HTTPClient` returning canned JSON — valid action passes through; unknown id → action dropped, suggestion kept; marker rendering with/without IDs.

### 5. `pkg/atseval`: the revise engine — `pkg/atseval/revise.go` (new)

```go
type RevisionContext struct{ Company, Role, Name string }

type RevisionRequest struct {
	JobDescription string
	SuggestionText string
	ActionType     string // "rewrite_field"
	Field          string // "summary" | "bullets" | "description"
	Content        string // current field content (HTML allowed)
	Context        RevisionContext
	Feedback       string // non-empty on retry: the guardrail violation to fix
	// Answers map[string]string — Phase 2
}

type RevisionResult struct {
	After     string
	Rationale string
}

func (e *Evaluator) Revise(ctx context.Context, req RevisionRequest) (*RevisionResult, error)
```

Reuses the existing `client.go` machinery (json_schema strict → json_object fallback, one parse retry, `ErrProvider`/`ErrBadResponse`). Response schema: `{"after": string, "rationale": string}`, both required, additionalProperties false.

Revise system prompt (new, in `prompt.go` or `revise.go`):

> You rewrite exactly one resume field. Rules:
> 1. Output the same structural kind you received — bullet list in (`<ul><li>…`), bullet list out; paragraph in, paragraph out. Use only these tags: p, ul, ol, li, strong, em, br.
> 2. NEVER add numbers, metrics, percentages, team sizes, dates, tool names, or achievements that are not present in the original text. You may rephrase, reorder, tighten, and strengthen verbs.
> 3. Weave in terminology from the job description only where the original content already supports it truthfully.
> 4. Keep roughly the original length (±40%).
> 5. Return JSON only: {"after": "...", "rationale": "..."} — rationale is one sentence describing what you changed.

User prompt layout:

```
# Job description
<jd>

# Suggestion to apply
<suggestion text>

# Field being rewritten (<field>, context: <company/role/name if present>)
<content>

# Previous attempt was rejected — fix this violation   ← only when Feedback != ""
<feedback>
```

Tests: fake HTTP client — happy path; feedback block present on retry request body; provider 500 → `ErrProvider`; garbage JSON twice → `ErrBadResponse`.

### 6. Application: `application/revision/usecase.go` (new)

Consumer-defined interface, declared here per house rules:

```go
type ResumeReviser interface {
	Revise(ctx context.Context, spec model.RevisionSpec) (draftAfter string, rationale string, err error)
}
```

(Define `model.RevisionSpec{JobDescription, SuggestionText string, Action SuggestionAction, Content string, Context RevisionContext, Feedback string}` in `domain/model/revision.go` — it crosses the application↔adapter boundary so it must live in domain, not adapter.)

```go
type Input struct {
	JobDescription string
	Suggestion     model.Suggestion // must carry a non-nil Action
	TargetField    string
	TargetContent  string
	TargetContext  model.RevisionContext
}

func (in Input) Validate() error
// - JobDescription non-empty            → ErrEmptyJobDescription (reuse)
// - Suggestion.Action non-nil, type == ActionRewriteField → ErrUnknownAction
// - ValidTargetCombo(action.Target)     → ErrUnknownAction
// - TargetField == action.Target.Field  → ErrTargetMismatch
// - TargetContent non-empty after trim  → ErrEmptyTargetContent
// - len(TargetContent) ≤ 20_000 bytes   → ErrTargetMismatch (or dedicated msg)

type Output struct{ Revision *model.Revision } // GetStatus() "ok"
```

`Execute` orchestration — **the retry policy lives here**, not in the adapter:

```
spec := build from input
draft, rationale, err := reviser.Revise(ctx, spec)      // err → ErrRevisionFailed passthrough
after := service.SanitizeHTML(draft)
verr  := first-of( service.ValidateShape(field, before, after),
                   service.CheckNoNewNumbers(before, after, nil) )
if verr != nil:
    spec.Feedback = verr.Error()
    draft, rationale, err = reviser.Revise(ctx, spec)    // exactly one retry
    re-sanitize + re-validate; still bad → return fmt.Errorf("%w: %v", model.ErrRevisionFailed, verr)
return Revision{Changes: [{Target: action.Target, Before: input content, After: after, Rationale: rationale}]}
```

Unit tests with mockery `ResumeReviser`: happy path; guardrail failure → second call carries Feedback → success; double failure → `ErrRevisionFailed`; every `Validate()` branch (table-driven).

### 7. Adapter: `adapter/llm/atseval/adapter.go`

Extend the consumer-declared `Engine` interface with `Revise(ctx, atseval.RevisionRequest) (*atseval.RevisionResult, error)`. Add `Adapter.Revise` mapping `model.RevisionSpec` → `atseval.RevisionRequest`; wrap `ErrProvider`/`ErrBadResponse` as `model.ErrRevisionFailed` (log details via slog, same pattern as Evaluate). Also map the new item IDs when converting `model.Resume` → `atseval.Resume` in the existing Evaluate path.

### 8. Adapter: stub reviser — `adapter/evaluator/stub/stub.go`

`StubReviser` (or extend the existing stub type) implementing `revision.ResumeReviser`, fully deterministic so drafted development and diff UI never need an API key:

- `bullets`: insert `<li>[stub edit] <suggestion text></li>` before the closing `</ul>`.
- `summary`/`description`: append `<p>[stub edit] <suggestion text></p>`.
- Rationale: `"stub reviser: deterministic edit for local development"`.

### 9. API layer

**`api/dto/revision.go` (new).** `RevisionRequest{JobDescription, Suggestion SuggestionDTO, Target RevisionTargetSliceDTO}` where `RevisionTargetSliceDTO{Field, Content string; Context RevisionContextDTO}` and `RevisionContextDTO{Company, Role, Name string}` (all `omitempty`). Response: `RevisionResponse{Status string; Changes []ChangeDTO; Warnings []string}` with `ChangeDTO{Target ActionTargetDTO; Before, After, Rationale string}`. Mapping helpers both directions.

**`api/dto/evaluation.go`.** `ExperienceDTO`/`ProjectDTO`/`EducationDTO`/`SkillGroupDTO` gain `ID string \`json:"id,omitempty"\``; `ToModel()` applies the ID hygiene rule; `SuggestionDTO` gains `Action *SuggestionActionDTO \`json:"action,omitempty"\`` with `SuggestionActionDTO{Type string; Target ActionTargetDTO}` and `ActionTargetDTO{Section string; ItemID string \`json:"item_id"\`; Field string}`; `NewEvaluationResponse` maps it.

**`api/http/handle/revision.go` (new).** Same shape as the evaluation handler: consumer interface `RevisionUseCase { Execute(ctx, core.Input) (core.Output, error) }`, 1 MB `http.MaxBytesReader`, decode → 10001, `errors.Is` mapping: `ErrEmptyJobDescription | ErrUnknownAction | ErrTargetMismatch | ErrEmptyTargetContent` → 400/10002 · `ErrRevisionFailed` → 503/30003 · default → 500/30001.

**`api/error_code/`.** Add `30003` (503, "revision service unavailable").

**`api/http/router.go`.** `POST /v1/revisions` wrapped in a second `RateLimitPerIP` instance driven by the new config value. `/health` untouched.

### 10. Config

`config/config.go`: `RateLimitRevisionsPerIPRPM int`, env `RATE_LIMIT_REVISIONS_PER_IP_RPM`, default **20**, `0` disables. Add the row to the README env table.

### 11. Wiring + mocks

- `adapter/dependency/providers.go`: `ProvideReviser(cfg *config.Config, logger *slog.Logger) revision.ResumeReviser` — mirrors `ProvideEvaluator`: stub when `cfg.LLMAPIKey == ""`, else `atseval.New(...)` + adapter. (Keep the two providers self-contained; sharing one engine instance is a refactor with no behavioral payoff — the engine is a stateless struct around `http.Client`.)
- `adapter/dependency/wire.go`: providers for `revision.NewUseCase`, `handle.NewRevisionHandler`, `wire.Bind` for `handle.RevisionUseCase`; router constructor gains the new handler. Then `make wire`, commit `wire.go` + `wire_gen.go` together.
- `.mockery.yaml`: register `application/revision.ResumeReviser` and `api/http/handle.RevisionUseCase` → `make mocks`, commit generated files.

### 12. Backend commit sequence

1. `feat(domain): resume item IDs, revision model, guardrail service` (+ tests)
2. `feat(atseval): item markers and suggestion actions in evaluation` (+ tests)
3. `feat(atseval): revise engine` (+ tests)
4. `feat(application): revision use case with guardrail retry` (+ mockery config, mocks, tests)
5. `feat(adapter): reviser adapter and deterministic stub`
6. `feat(api): /v1/revisions endpoint, DTOs, error code 30003, rate-limit config`
7. `chore(wire): wire revision use case and handler` (`wire.go` + `wire_gen.go`)

`make all` green at every step.

---

## Frontend (drafted)

### 1. Stable item IDs

- `src/types.ts`: add `id: string` to `Experience`, `Project`, `Education`, `SkillGroup`.
- ID factory: `crypto.randomUUID()` with a `Math.random().toString(36)` fallback (older Safari). Trim to ≤64 chars — backend blanks anything longer/odd.
- `src/normalize.ts normalizeResumeData()`: assign `id` where missing (idempotent — this is the localStorage migration path; persisted on next debounced autosave).
- `addItem` in `src/ResumeBuilder.tsx`: new items get an `id`.
- Replace `key={i}` with `key={item.id}` in the editor list renders for these four lists (also fixes drag/reorder reconciliation). `reorder`/`removeItem`/`setItemField` stay index-based — unchanged.
- `src/evaluation.ts toEvaluationRequest()`: include `id` for each item.

### 2. API client — `src/evaluation.ts`

- `EvaluationSuggestion` gains `action?: { type: 'rewrite_field'; target: { section: string; item_id: string; field: string } }`.
- New types `RevisionChange`, `RevisionResponse`; new `reviseResume(params, signal)` posting to `${VITE_API_BASE_URL ?? ''}/v1/evaluations`-style base + `/v1/revisions`, reusing `EvaluationError` and the error envelope handling.
- `FRIENDLY_MESSAGES`: add `30003: "Couldn't generate a safe edit right now — try again in a moment."`
- Chore: fix the stale "forged has no CORS middleware" comment in `vite.config.ts` (CORS shipped backend-side).

### 3. Target resolution — new pure module `src/revisionTarget.ts` (vitest-tested)

```ts
resolveTarget(state: ResumeData, target: {section; item_id; field}):
  | { content: string; context: {company?; role?; name?} }
  | null
// summary            → { content: state.summary, context: { role: state.targetRole || undefined } }
// experience + id    → { content: exp.bulletsText, context: { company, role } }
// projects + id      → { content: proj.description, context: { name } }
// unknown id / combo → null   (stale target — resume changed since evaluation)

applyChange(state, change): Partial<State> | { list: ListKey; index: number; field: string; value: string }
// summary    → patch({ summary: change.after })
// experience → setItemField('experience', i, 'bulletsText', change.after)
// projects   → setItemField('projects',  i, 'description', change.after)

suggestionKey(s: EvaluationSuggestion): string
// stable identity for applied-tracking: `${s.section}|${s.action?.target.item_id ?? ''}|${s.text}`
```

### 4. `SuggestedEditsCard` (`src/JobMatch.tsx`) becomes interactive

Row states: `idle → loading → (preview) → applied | error`, plus `stale` when `resolveTarget` returns null.

- **Apply** button only when `suggestion.action` exists and target resolves; otherwise keep today's row exactly (badge + text + jump-to-section, which stays on every row).
- One revise call in flight at a time — other Apply buttons disabled while loading (simplest correct state model; revisit in Phase 3 batching).
- `stale` rows show "Your resume changed here — re-run the match" (disabled button).
- Error rows show the friendly message inline with a retry affordance.
- Applied rows: check mark + strikethrough; excluded from the "N pts left on the table" figure in `matchScore.ts pointsLeft` (pass applied keys in).

### 5. Diff preview modal — new `src/ReviseModal.tsx`

- Stacked **Before** / **After** panels rendering the HTML read-only (`dangerouslySetInnerHTML` is acceptable *only* because the backend sanitized `after` against the tag allowlist and `before` is the user's own Tiptap output; note this in a comment), plus the one-line rationale.
- Buttons: **Apply change** / **Discard**. Esc/backdrop = discard. Reuse existing modal/toast patterns and `var(--c-*)` tokens from `src/styles.ts`.

### 6. Accept → apply + undo (`src/ResumeBuilder.tsx`)

- Before applying: push a deep-cloned `ResumeData` snapshot (the `PERSIST_FIELDS` slice) onto an undo stack — `useRef<ResumeData[]>`, capped at 20.
- Apply via the existing `patch` / `setItemField` helpers (per `applyChange`).
- Toast: "Edit applied — **Undo**". Undo pops the snapshot and restores wholesale (autosave then persists it).
- Record `suggestionKey` in applied set; persist the set inside the existing `drafted:match:v1` report so applied marks survive reload; clear it whenever a new evaluation result arrives.

### 7. Re-score nudge

After ≥1 successful apply, show a banner in the match report: "You've applied N edits — re-run the match to see your new score" → `runMatch()`. The existing `evalPrevScore` delta chip then shows realized lift; no new score UI needed in this phase.

### 8. Frontend commit sequence

1. `feat: stable item ids with localStorage migration and keyed lists`
2. `feat: suggestion actions in evaluation types + reviseResume client`
3. `feat: apply flow — resolve target, diff modal, apply with undo`
4. `feat: applied tracking and re-run-match nudge`

---

## Test matrix (definition of done)

| # | Scenario | Expected |
|---|---|---|
| 1 | Apply a bullets rewrite with stub (no `LLM_API_KEY`) | Diff modal shows deterministic `[stub edit]` change; accept updates the entry; undo restores |
| 2 | Apply with real LLM key | `after` passes guardrails; only tags from the allowlist; no new numbers vs `before` |
| 3 | LLM adds a fabricated number | Use case retries once with feedback; if still bad → 503/30003; FE shows friendly message |
| 4 | User reorders/deletes entries between evaluate and apply | Row goes `stale`; wrong item is never modified (ID-addressed, not index-addressed) |
| 5 | Suggestion without `action` (stub evaluator, or parser dropped it) | Row renders exactly as today — display-only |
| 6 | Oversize `target.content` (>20 KB) or field mismatch | 400/10002 |
| 7 | Rate limit exceeded on `/v1/revisions` | 429/10003, friendly message |
| 8 | Old localStorage data (no ids) | `normalizeResumeData` assigns ids once; evaluation request includes them |
| 9 | `/health` behavior | Untouched, still unconditional 200 |
| 10 | `make all` + CI wire freshness | Green; `wire_gen.go` current; mocks committed |

## Risks / accepted limitations

- **Semantic mis-targeting:** the parser verifies the target *exists*, not that it's the *best* one — the diff preview + explicit accept is the safety net.
- **Numeric-only fact check:** Phase 1 guardrails catch fabricated numbers, not fabricated tool names or claims; the prompt forbids them and the user reviews every diff. Phase 2 tightens this around answers.
- **Locale/format drift** ("2M" vs "2 million"): literal token matching may reject valid rephrasings → retry → occasional 503. Acceptable; log guardrail failures (slog, no content) to tune the regex later.
