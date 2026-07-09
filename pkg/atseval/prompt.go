package atseval

import (
	"fmt"
	"strings"
)

// systemPrompt defines the evaluator persona and the JD-match rubric.
// Point bands and hard rules follow the style of HackerRank's hiring-agent
// evaluation prompts, adapted to score against a job description.
const systemPrompt = `You are a rigorous, fair technical recruiter evaluating how well a candidate's resume matches a specific job description. You apply an objective rubric and never invent facts that are not in the resume.

Score the resume against the job description on exactly these four dimensions:

1. skills_match (0-35) — "Skills match"
   High (25-35): the resume shows hands-on evidence (work bullets, projects) for most of the skills and technologies the job description requires.
   Medium (13-24): several required skills are evidenced; others are missing or only listed without supporting evidence.
   Low (0-12): few or none of the required skills appear with evidence.

2. experience_relevance (0-30) — "Experience relevance"
   High (21-30): roles, domains, and seniority closely match what the job description asks for.
   Medium (11-20): adjacent roles or partial seniority match.
   Low (0-10): experience is largely unrelated to the role.

3. impact_evidence (0-20) — "Impact and evidence"
   High (14-20): bullets are specific and quantified (metrics, scale, outcomes) and relate to the job's responsibilities.
   Medium (7-13): some concrete outcomes, mostly duty descriptions.
   Low (0-6): vague, generic, or unverifiable claims.

4. education_extras (0-15) — "Education and extras"
   High (11-15): education and extras (certifications, links, portfolio, open source) satisfy the job's stated requirements and add relevant signal.
   Medium (6-10): partially relevant.
   Low (0-5): missing or irrelevant to this job.

Hard rules:
- Score ONLY against the provided job description. A strong resume for a different job scores low here.
- Every point you award must be backed by evidence quoted or paraphrased from the resume. Put that evidence in the dimension's "evidence" field.
- A skill merely listed in a skills section with no supporting experience or project evidence scores in the low band for that skill.
- Ignore the candidate's name, demographics, institution prestige, GPA, and location. Never mention them.
- "gaps" are requirements from the job description the resume does not demonstrate.
- Provide 3-6 "suggestions": concrete, actionable resume edits referencing specific bullets or sections of this resume (e.g. "Quantify the Acme migration bullet with request volume or latency numbers"), never generic advice. For each suggestion:
  - "section" is the resume section the user should edit: one of "summary", "experience", "projects", "education", "skills".
  - "dimension" is the rubric dimension the edit improves: one of "skills_match", "experience_relevance", "impact_evidence", "education_extras".
  - "estimated_lift" is a realistic estimate of the score points gained if the edit is applied. The sum of estimated_lift values for a given dimension must not exceed that dimension's remaining headroom (its max minus the score you awarded).
  - Order suggestions by estimated_lift, highest first.
  - When a suggestion targets one specific entry, include an "action" of type "rewrite_field". The "target" MUST include all three fields: "section", "item_id", and "field". The "target.item_id" MUST be the BARE id only — strip the prefix and brackets entirely (e.g. "[exp:a1b2c3]" → "a1b2c3", NEVER "exp:a1b2c3"). Valid targets: section "summary" with field "summary" and empty item_id; section "experience" with field "bullets"; section "projects" with field "description". If a suggestion has no single target, set "action" to null.
- The overall "score" must equal the sum of the four dimension scores.

Respond with a single JSON object exactly matching this shape (no markdown, no commentary):
{
  "score": <int 0-100>,
  "summary": "<2-3 sentence overall assessment>",
  "dimensions": [
    {"key": "skills_match", "label": "Skills match", "score": <int>, "max": 35, "evidence": "..."},
    {"key": "experience_relevance", "label": "Experience relevance", "score": <int>, "max": 30, "evidence": "..."},
    {"key": "impact_evidence", "label": "Impact and evidence", "score": <int>, "max": 20, "evidence": "..."},
    {"key": "education_extras", "label": "Education and extras", "score": <int>, "max": 15, "evidence": "..."}
  ],
  "strengths": ["..."],
  "gaps": ["..."],
  "suggestions": [
    {"text": "...", "section": "experience", "dimension": "impact_evidence", "estimated_lift": 4, "action": {"type": "rewrite_field", "target": {"section": "experience", "item_id": "a1b2c3", "field": "bullets"}}},
    {"text": "...", "section": "skills", "dimension": "skills_match", "estimated_lift": 2, "action": null}
  ]
}`

// userPrompt renders the job description and resume into the user message.
func userPrompt(jobDescription string, resume Resume) string {
	var b strings.Builder
	b.WriteString("# Job description\n\n")
	b.WriteString(strings.TrimSpace(jobDescription))
	b.WriteString("\n\n# Resume\n\n")
	b.WriteString(renderResume(resume))
	return b.String()
}

// renderResume converts a structured resume into readable markdown,
// stripping any HTML markup from rich-text fields.
func renderResume(r Resume) string {
	var b strings.Builder

	if s := strings.TrimSpace(stripHTML(r.Summary)); s != "" {
		b.WriteString("## Summary\n")
		b.WriteString(s)
		b.WriteString("\n\n")
	}

	if len(r.Experience) > 0 {
		b.WriteString("## Experience\n")
		for _, e := range r.Experience {
			end := e.End
			if e.Present {
				end = "present"
			}
			if e.ID != "" {
				fmt.Fprintf(&b, "### [exp:%s] %s — %s", e.ID, e.Role, e.Company)
			} else {
				fmt.Fprintf(&b, "### %s — %s", e.Role, e.Company)
			}
			if e.Employment != "" {
				fmt.Fprintf(&b, " (%s)", e.Employment)
			}
			if e.Start != "" || end != "" {
				fmt.Fprintf(&b, " | %s - %s", e.Start, end)
			}
			b.WriteString("\n")
			if bullets := strings.TrimSpace(stripHTML(e.Bullets)); bullets != "" {
				b.WriteString(bullets)
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}

	if len(r.Projects) > 0 {
		b.WriteString("## Projects\n")
		for _, p := range r.Projects {
			if p.ID != "" {
				fmt.Fprintf(&b, "### [prj:%s] %s", p.ID, p.Name)
			} else {
				fmt.Fprintf(&b, "### %s", p.Name)
			}
			if p.Link != "" {
				fmt.Fprintf(&b, " (%s)", p.Link)
			}
			b.WriteString("\n")
			if d := strings.TrimSpace(stripHTML(p.Description)); d != "" {
				b.WriteString(d)
				b.WriteString("\n")
			}
			if len(p.TechStack) > 0 {
				fmt.Fprintf(&b, "Tech stack: %s\n", strings.Join(p.TechStack, ", "))
			}
			b.WriteString("\n")
		}
	}

	if len(r.Education) > 0 {
		b.WriteString("## Education\n")
		for _, ed := range r.Education {
			if ed.ID != "" {
				fmt.Fprintf(&b, "### [edu:%s] %s — %s", ed.ID, ed.Degree, ed.School)
			} else {
				fmt.Fprintf(&b, "### %s — %s", ed.Degree, ed.School)
			}
			if ed.Start != "" || ed.End != "" {
				fmt.Fprintf(&b, " | %s - %s", ed.Start, ed.End)
			}
			b.WriteString("\n")
			for _, d := range ed.ExtraDetails {
				fmt.Fprintf(&b, "- %s: %s\n", d.Label, d.Value)
			}
			b.WriteString("\n")
		}
	}

	if len(r.SkillGroups) > 0 {
		b.WriteString("## Skills\n")
		for _, sg := range r.SkillGroups {
			if sg.ID != "" {
				fmt.Fprintf(&b, "- [skill:%s] %s: %s\n", sg.ID, sg.Label, strings.Join(sg.Items, ", "))
			} else {
				fmt.Fprintf(&b, "- %s: %s\n", sg.Label, strings.Join(sg.Items, ", "))
			}
		}
		b.WriteString("\n")
	}

	links := make([]string, 0, 2)
	if r.LinkedIn != "" {
		links = append(links, "LinkedIn: "+r.LinkedIn)
	}
	if len(links) > 0 {
		b.WriteString("## Links\n")
		b.WriteString(strings.Join(links, "\n"))
		b.WriteString("\n")
	}

	return b.String()
}

// stripHTML removes HTML tags from rich-text editor output, turning list
// items into "- " bullets and paragraph/line breaks into newlines.
func stripHTML(s string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	inTag := false
	var tag strings.Builder
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
			tag.Reset()
		case r == '>' && inTag:
			inTag = false
			raw := tag.String()
			closing := strings.HasPrefix(raw, "/")
			name := strings.TrimPrefix(raw, "/")
			if i := strings.IndexAny(name, " \t\n/"); i >= 0 {
				name = name[:i]
			}
			name = strings.ToLower(name)
			switch {
			case name == "li" && !closing:
				b.WriteString("\n- ")
			case name == "br" || name == "p" || name == "ul" || name == "ol" || name == "div":
				// Block tags become line breaks; blank runs collapse below.
				b.WriteString("\n")
			}
		case inTag:
			tag.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	// Collapse runs of blank lines produced by adjacent block tags.
	lines := strings.Split(b.String(), "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return strings.Join(out, "\n")
}
