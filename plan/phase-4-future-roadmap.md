# Phase 4 — Future Roadmap

**Status:** directional (revisit after Phase 3 ships) · **Parent:** [interactive-suggestions.md](interactive-suggestions.md)

Everything here builds on the Phase 1–3 foundation without changing the `/v1/revisions` contract. Items are grouped by what they cost: the first tier needs no new infrastructure; the second tier is gated on adding accounts + a database; the third is product direction. Within each tier, roughly in recommended order.

---

## Tier 1 — No new infrastructure

### 1. Rewrite variants
`Revise` returns 2–3 alternatives (`after` becomes `variants[]` behind a request flag); the diff modal becomes a picker. Cheap to build (schema + prompt + modal), high perceived quality — users stop rejecting rewrites over tone. Guardrails run per variant; drop variants that fail rather than retrying each.

### 2. Auto-fix mode
"Fix everything you can" = Phase 3 batch apply promoted to a single primary action after every evaluation, ending in one combined review screen. Purely a UX composition of existing pieces; ship once batch telemetry (Phase 3 counters) shows applies usually get accepted.

### 3. Evaluation caching
In-memory LRU keyed by `hash(normalized resume + JD + model)` with a short TTL (~1h) to absorb repeat runs — users re-score constantly in the apply loop. Adapter-layer concern (`adapter/` cache wrapping the evaluator, selected in `providers.go`); stateless deploys tolerate cache loss by design. Skip Redis; a Railway redeploy clearing the cache is fine.

### 4. Deterministic keyword coverage
A non-LLM JD-term extraction (`pkg/atseval`, stdlib) rendering a "covered / missing" keyword table alongside the rubric. Instant, free, runs even in stub mode — makes the free tier feel alive and gives `add_skill` suggestions corroborating evidence.

### 5. Cover letter / outreach generation
Same engine, same guardrail philosophy: facts only from resume + user answers, HTML-sanitized, previewed before use. New use case + endpoint (`/v1/letters`), new prompt; drafted gains a tab. The revision guardrail suite is reused nearly verbatim — that's the point of having built it in domain/service.

### 6. Structure-level suggestions
Display-only advice the FE executes locally with existing `reorder`/`removeItem` helpers ("move Projects above Education for this JD", "your summary is 3× typical length"). New action types executed client-side without a revise call — no backend generation, so no guardrail surface.

## Tier 2 — The accounts era (DB becomes justified)

**Trigger:** real demand for cross-device sync, history, or sharing — never payload size (see the overview's stateless rationale). When triggered, adopt in one step: **auth + Postgres on Railway**, with `adapter/repository/` finally earning its keep.

- **Accounts & stored resumes.** Managed auth (e.g. Clerk/Auth0 — rolling our own sessions is not the business we're in) + minimal schema:
  `users(id, email, created_at)` · `resumes(id, user_id, title, data jsonb, updated_at)` · `resume_versions(id, resume_id, data jsonb, cause, created_at)` · `evaluations(id, resume_version_id, jd_hash, result jsonb, created_at)`.
  Every accepted revision writes a `resume_versions` row with `cause = suggestion key` — version history and "restore point before auto-fix" fall out for free.
- **Sync model.** Client stays offline-first (localStorage remains source of truth mid-session); server holds checkpoints, last-write-wins with updated_at, surfaced conflicts only on explicit restore. The `/v1/revisions` slice contract is unchanged; the server merely gains the *option* to resolve `target.content` from a stored version.
- **Shareable evaluation links.** `/e/<id>` renders a read-only score report — the first genuinely viral surface. Requires stored evaluations + an unauthenticated read path with expiry.
- **Per-user quotas.** Replace per-IP rate limiting with per-user metering; this is also the billing seam (free: N evaluations + M revisions/month). Middleware swap, config-driven.
- **Server-side analytics.** Phase 3's local counters graduate to an events table; prompt tuning finally gets real accept/dismiss data across users.
- **Obligations arriving with this tier (budget for them, don't bolt on):** retention policy, account deletion that actually deletes, encryption at rest, a privacy page that stops saying "we never store your resume" and starts saying what we do store.

## Tier 3 — Platform direction

- **Multi-JD tracker.** Store several JDs per resume; matrix view "which role am I closest to?"; per-JD suggestion sets. Mostly FE + storage; evaluation engine unchanged.
- **Interview-prep pack.** Generated from the weakest dimensions and gap list ("impact evidence is your gap — prepare these three stories, quantified with the metrics you gave us"). Reuses answers collected in Phase 2 as story seeds.
- **Template-aware suggestions.** drafted already exports PDF/LaTeX/Word; suggestions could account for format constraints (one-page pressure, LaTeX-safe characters). Low urgency until users ask.
- **Job-description ingestion.** Paste a URL instead of text (server-side fetch + readability extraction) — small backend feature, large UX smoothing for the top of the funnel.

## Sequencing sanity check

Recommended order after Phase 3: **1.1 variants → 1.3 caching → 1.4 keywords → 1.2 auto-fix**, then reassess whether Tier 2's trigger has fired. Tier 1 items are each ≤ a few days and independently shippable; Tier 2 is a program, not a feature — enter it deliberately, with the schema above as the starting point, not the endpoint.
