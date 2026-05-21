package workflowdefs

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/multica-ai/multica/server/internal/execprotocol"
)

const (
	SystemStandardAssignment = "standard-assignment"
	SystemTrellisTask        = "trellis-task"
	SystemDirectTask         = "direct-task"
	SystemResearchNote       = "research-note"
	SystemCommentResponse    = "comment-response"
)

type Seed struct {
	Key         string
	Name        string
	Description string
	Schema      []byte
}

type Schema struct {
	SchemaVersion int        `json:"schema_version,omitempty"`
	Version       int        `json:"version,omitempty"`
	Name          string     `json:"name,omitempty"`
	Description   string     `json:"description,omitempty"`
	Category      string     `json:"category,omitempty"`
	Applicability []string   `json:"applicability,omitempty"`
	Source        Source     `json:"source,omitempty"`
	Variables     []Variable `json:"variables,omitempty"`
	Steps         []Step     `json:"steps,omitempty"`
	Gates         []Gate     `json:"gates,omitempty"`
}

type Source struct {
	Format       string `json:"format,omitempty"`
	Mode         string `json:"mode,omitempty"`
	BodyTemplate string `json:"body_template,omitempty"`
}

type Variable struct {
	Key         string `json:"key"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type Step struct {
	ID             string           `json:"id"`
	Name           string           `json:"name,omitempty"`
	Title          string           `json:"title"`
	Order          int              `json:"order,omitempty"`
	Required       *bool            `json:"required,omitempty"`
	DependsOn      []string         `json:"depends_on,omitempty"`
	Execution      *StepExecution   `json:"execution,omitempty"`
	Output         *StepOutput      `json:"output,omitempty"`
	Artifact       *StepArtifact    `json:"artifact,omitempty"`
	InputArtifacts []ArtifactInput  `json:"input_artifacts,omitempty"`
	Review         *StepReview      `json:"review,omitempty"`
	ReviewRequired *bool            `json:"review_required,omitempty"`
	QualityGate    *StepQualityGate `json:"quality_gate,omitempty"`
	BodyTemplate   string           `json:"body_template,omitempty"`
	Description    string           `json:"description,omitempty"`
	Checklist      []string         `json:"checklist,omitempty"`
}

type StepExecution struct {
	Kind   string `json:"kind,omitempty"`
	Prompt string `json:"prompt,omitempty"`
	Rules  string `json:"rules,omitempty"`
}

type StepOutput struct {
	Description string `json:"description,omitempty"`
}

type StepArtifact struct {
	Name        string            `json:"name,omitempty"`
	ContentKind string            `json:"content_kind,omitempty"`
	Format      string            `json:"format,omitempty"`
	Template    *ArtifactTemplate `json:"template,omitempty"`
	Inputs      []ArtifactInput   `json:"inputs,omitempty"`
	Description string            `json:"description,omitempty"`
}

type ArtifactTemplate struct {
	Format  string                 `json:"format,omitempty"`
	Content string                 `json:"content,omitempty"`
	Files   []ArtifactTemplateFile `json:"files,omitempty"`
}

func (t *ArtifactTemplate) UnmarshalJSON(data []byte) error {
	var content string
	if err := json.Unmarshal(data, &content); err == nil {
		t.Content = content
		return nil
	}
	type alias ArtifactTemplate
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*t = ArtifactTemplate(decoded)
	return nil
}

type ArtifactTemplateFile struct {
	Path    string `json:"path,omitempty"`
	Content string `json:"content,omitempty"`
}

type ArtifactInput struct {
	StepID       string `json:"step_id,omitempty"`
	ArtifactName string `json:"artifact_name,omitempty"`
	Name         string `json:"name,omitempty"`
	Required     bool   `json:"required,omitempty"`
}

type StepReview struct {
	Required     bool   `json:"required,omitempty"`
	ReviewerRole string `json:"reviewer_role,omitempty"`
	Instructions string `json:"instructions,omitempty"`
}

type StepQualityGate struct {
	Enabled    bool   `json:"enabled,omitempty"`
	Blocking   bool   `json:"blocking,omitempty"`
	Prompt     string `json:"prompt,omitempty"`
	ReportMode string `json:"report_mode,omitempty"`
}

type Gate struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

type RenderContext struct {
	IssueID          string
	TriggerCommentID string
}

type RenderResult struct {
	Markdown string   `json:"rendered_markdown"`
	Warnings []string `json:"warnings"`
}

type ValidationIssue struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

type ValidationResult struct {
	Publishable bool              `json:"publishable"`
	Issues      []ValidationIssue `json:"issues"`
}

func SystemSeeds() []Seed {
	standard, _ := execprotocol.Get(execprotocol.StandardAssignmentSlug)
	trellis, _ := execprotocol.Get(execprotocol.TrellisTaskSlug)

	return []Seed{
		seedFromTemplate(SystemStandardAssignment, standard, []string{"assignment"}),
		seedFromTemplate(SystemTrellisTask, trellis, []string{"assignment"}),
		systemSeed(
			SystemDirectTask,
			"Direct task",
			"Short assignment workflow for small bugs and single-step changes.",
			[]string{"assignment"},
			`
## Direct Task Protocol

Use this workflow for small, well-scoped bugs or single-step tasks.

1. Run `+"`multica issue get {{issue_id}} --output json`"+` and `+"`multica issue comment list {{issue_id}} --output json`"+`.
2. Confirm the request is already clear enough to act on. If not, post the missing question and mark the issue blocked.
3. Make the smallest change that addresses the request.
4. Run targeted verification. If verification cannot run, record why.
5. Post a concise final comment and move the issue to `+"`in_review`"+`.
`,
			[]Step{
				{ID: "context", Title: "Load issue context", Order: 1, Description: "Read the issue and latest comments before acting."},
				{ID: "clarity", Title: "Confirm clarity", Order: 2, DependsOn: []string{"context"}, Description: "Proceed only when the request is already clear enough."},
				{ID: "execute", Title: "Make the smallest change", Order: 3, DependsOn: []string{"clarity"}, Description: "Apply the narrow fix or single-step change."},
				{ID: "verify", Title: "Run targeted verification", Order: 4, DependsOn: []string{"execute"}, Description: "Run focused checks and record any verification gaps."},
				{ID: "report", Title: "Post final comment", Order: 5, DependsOn: []string{"verify"}, Description: "Summarize outcome and move the issue to review."},
			},
		),
		systemSeed(
			SystemResearchNote,
			"Research note",
			"Investigation workflow that returns findings without forcing implementation phases.",
			[]string{"assignment"},
			`
## Research Note Protocol

Use this workflow when the deliverable is analysis, options, or a recommendation.

1. Run `+"`multica issue get {{issue_id}} --output json`"+` and `+"`multica issue comment list {{issue_id}} --output json`"+`.
2. Identify the question, constraints, and what evidence would change the answer.
3. Inspect the relevant repository, resources, documentation, or runtime state.
4. Post findings as a structured issue comment: answer, evidence, tradeoffs, and recommended next action.
5. Move the issue to `+"`in_review`"+` unless more input is needed; if blocked, state the exact missing input.
`,
			[]Step{
				{ID: "context", Title: "Load issue context", Order: 1, Description: "Read the issue and full comment history."},
				{ID: "frame", Title: "Frame the question", Order: 2, DependsOn: []string{"context"}, Description: "Identify the question, constraints, and useful evidence."},
				{ID: "inspect", Title: "Inspect evidence", Order: 3, DependsOn: []string{"frame"}, Description: "Read the relevant repositories, resources, docs, or runtime state."},
				{ID: "findings", Title: "Post findings", Order: 4, DependsOn: []string{"inspect"}, Description: "Return answer, evidence, tradeoffs, and recommended next action."},
				{ID: "close", Title: "Set final status", Order: 5, DependsOn: []string{"findings"}, Description: "Move to review or block with exact missing input."},
			},
		),
		systemSeed(
			SystemCommentResponse,
			"Comment response",
			"Focused workflow for @mention/comment-triggered agent replies.",
			[]string{"comment"},
			`
## Comment Response Protocol

This task was triggered by a new issue comment.

1. Run `+"`multica issue get {{issue_id}} --output json`"+` and `+"`multica issue comment list {{issue_id}} --output json`"+`.
2. Find the triggering comment `+"`{{trigger_comment_id}}`"+` and answer that comment, not an older thread.
3. If the comment asks for work, do the work before replying. If it is only acknowledgement and no action is needed, exit silently.
4. If you reply, post it with `+"`multica issue comment add {{issue_id}} --content \"...\"`"+`. Terminal output is not delivered to the user.
5. Do not change issue status unless the comment explicitly asks for it.
`,
			[]Step{
				{ID: "context", Title: "Load issue context", Order: 1, Description: "Read the issue and full comment history."},
				{ID: "trigger", Title: "Find triggering comment", Order: 2, DependsOn: []string{"context"}, Description: "Answer the new comment, not an older thread."},
				{ID: "decide", Title: "Decide whether work is needed", Order: 3, DependsOn: []string{"trigger"}, Description: "Do the work if requested, or exit silently for pure acknowledgement."},
				{ID: "reply", Title: "Post response when needed", Order: 4, DependsOn: []string{"decide"}, Description: "Deliver the result via issue comment when a reply is warranted."},
				{ID: "status", Title: "Respect status discipline", Order: 5, DependsOn: []string{"reply"}, Description: "Change issue status only when explicitly requested."},
			},
		),
	}
}

func DefaultUserSchema(name, description string) []byte {
	s := Schema{
		SchemaVersion: 2,
		Name:          strings.TrimSpace(name),
		Description:   strings.TrimSpace(description),
		Applicability: []string{"assignment"},
		Source: Source{
			Format: "markdown",
			BodyTemplate: strings.TrimSpace(`
## Custom Workflow

1. Run `+"`multica issue get {{issue_id}} --output json`"+` and read the current task.
2. Run `+"`multica issue comment list {{issue_id}} --output json`"+` and incorporate the latest discussion.
3. Execute the requested work using the agent's skills and instructions.
4. Verify the outcome, post a final issue comment, and move the issue to `+"`in_review`"+`.
`) + "\n",
		},
		Steps: []Step{
			{ID: "context", Title: "Read current task", Order: 1, Description: "Load the issue and current task details."},
			{ID: "discussion", Title: "Read latest discussion", Order: 2, DependsOn: []string{"context"}, Description: "Incorporate the newest issue comments before acting."},
			{ID: "execute", Title: "Execute work", Order: 3, DependsOn: []string{"discussion"}, Description: "Complete the requested work using the agent's skills and instructions."},
			{ID: "verify", Title: "Verify and report", Order: 4, DependsOn: []string{"execute"}, Description: "Verify the outcome, comment the result, and move the issue to review."},
		},
	}
	raw, _ := json.Marshal(s)
	return raw
}

func NormalizeDraftSchema(raw []byte, fallbackName, fallbackDescription string) ([]byte, ValidationResult, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		raw = DefaultUserSchema(fallbackName, fallbackDescription)
	}

	var s Schema
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, ValidationResult{}, fmt.Errorf("schema must be a JSON object: %w", err)
	}
	if s.SchemaVersion == 0 && s.Version != 0 {
		s.SchemaVersion = s.Version
	}
	if s.SchemaVersion == 0 {
		s.SchemaVersion = 2
	}
	if s.SchemaVersion == 1 {
		s.SchemaVersion = 2
	}
	s.Version = 0
	if strings.TrimSpace(s.Name) == "" {
		s.Name = strings.TrimSpace(fallbackName)
	}
	if strings.TrimSpace(s.Description) == "" {
		s.Description = strings.TrimSpace(fallbackDescription)
	}
	if s.Source.Format == "" && s.Source.Mode != "" {
		s.Source.Format = s.Source.Mode
	}
	if s.Source.Format == "" {
		s.Source.Format = "markdown"
	}
	s.Source.Mode = ""
	normalizeSteps(s.Steps)
	normalized, err := json.Marshal(s)
	if err != nil {
		return nil, ValidationResult{}, fmt.Errorf("normalize schema: %w", err)
	}
	return normalized, ValidateSchema(normalized), nil
}

func NormalizeSchema(raw []byte, fallbackName, fallbackDescription string) ([]byte, error) {
	normalized, validation, err := NormalizeDraftSchema(raw, fallbackName, fallbackDescription)
	if err != nil {
		return nil, err
	}
	if !validation.Publishable {
		return nil, fmt.Errorf("%s", validation.Issues[0].Message)
	}
	return normalized, nil
}

func ValidateSchema(raw []byte) ValidationResult {
	result := ValidationResult{Publishable: true}
	addBlocking := func(code, message string) {
		result.Publishable = false
		result.Issues = append(result.Issues, ValidationIssue{
			Code:     code,
			Message:  message,
			Severity: "blocking",
		})
	}

	var s Schema
	if err := json.Unmarshal(raw, &s); err != nil {
		addBlocking("schema_parse_failed", "schema must be a JSON object")
		return result
	}
	if s.SchemaVersion != 1 && s.SchemaVersion != 2 {
		addBlocking("unsupported_schema_version", fmt.Sprintf("unsupported schema_version %d", s.SchemaVersion))
	}
	if len(s.Applicability) == 0 {
		addBlocking("missing_applicability", "schema applicability is required")
	}
	for _, item := range s.Applicability {
		if item != "assignment" && item != "comment" && item != "chat" && item != "autopilot" {
			addBlocking("unsupported_applicability", fmt.Sprintf("unsupported workflow applicability %q", item))
		}
	}
	normalizeSteps(s.Steps)
	if err := validateSteps(s.Steps); err != nil {
		addBlocking("invalid_steps", err.Error())
	}
	return result
}

func normalizeSteps(steps []Step) {
	for i := range steps {
		step := &steps[i]
		step.ID = strings.TrimSpace(step.ID)
		step.Name = strings.TrimSpace(step.Name)
		step.Title = strings.TrimSpace(step.Title)
		if step.Title == "" {
			step.Title = step.Name
		}
		if step.Title == "" {
			step.Title = step.ID
		}
		if step.Order == 0 {
			step.Order = i + 1
		}
		step.DependsOn = normalizedStrings(step.DependsOn)
		step.BodyTemplate = strings.TrimSpace(step.BodyTemplate)
		step.Description = strings.TrimSpace(step.Description)
		step.Checklist = normalizedStrings(step.Checklist)
		if step.Execution == nil {
			step.Execution = &StepExecution{Kind: "agent"}
		}
		step.Execution.Kind = normalizeExecutionKind(step.Execution.Kind)
		step.Execution.Prompt = strings.TrimSpace(step.Execution.Prompt)
		step.Execution.Rules = strings.TrimSpace(step.Execution.Rules)
		if len(step.InputArtifacts) > 0 {
			if step.Artifact == nil {
				step.Artifact = &StepArtifact{}
			}
			if len(step.Artifact.Inputs) == 0 {
				step.Artifact.Inputs = step.InputArtifacts
			}
			step.InputArtifacts = nil
		}
		if step.Artifact != nil {
			step.Artifact.Name = strings.TrimSpace(step.Artifact.Name)
			if step.Artifact.ContentKind == "" {
				step.Artifact.ContentKind = step.Artifact.Format
			}
			step.Artifact.ContentKind = normalizeArtifactContentKind(step.Artifact.ContentKind)
			step.Artifact.Format = ""
			step.Artifact.Description = strings.TrimSpace(step.Artifact.Description)
			if step.Artifact.Template != nil {
				if step.Artifact.Template.Format == "" {
					step.Artifact.Template.Format = step.Artifact.ContentKind
				}
				step.Artifact.Template.Format = normalizeArtifactContentKind(step.Artifact.Template.Format)
				step.Artifact.Template.Content = strings.TrimSpace(step.Artifact.Template.Content)
			}
			normalizeArtifactInputs(step.Artifact.Inputs)
		}
		if step.ReviewRequired != nil {
			if step.Review == nil {
				step.Review = &StepReview{}
			}
			step.Review.Required = *step.ReviewRequired
			step.ReviewRequired = nil
		}
		if step.QualityGate != nil {
			step.QualityGate.Prompt = strings.TrimSpace(step.QualityGate.Prompt)
			step.QualityGate.ReportMode = strings.TrimSpace(step.QualityGate.ReportMode)
			if step.QualityGate.ReportMode == "" {
				step.QualityGate.ReportMode = "summary"
			}
			if step.QualityGate.Prompt != "" {
				step.QualityGate.Enabled = true
			}
		}
	}
}

func normalizedStrings(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeArtifactInputs(inputs []ArtifactInput) {
	for i := range inputs {
		inputs[i].StepID = strings.TrimSpace(inputs[i].StepID)
		inputs[i].ArtifactName = strings.TrimSpace(inputs[i].ArtifactName)
		inputs[i].Name = strings.TrimSpace(inputs[i].Name)
	}
}

func normalizeExecutionKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "agent", "built_in_agent", "local_agent":
		return "agent"
	case "manual":
		return "manual"
	case "external", "external_agent":
		return "external"
	default:
		return strings.ToLower(strings.TrimSpace(kind))
	}
}

func normalizeArtifactContentKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "text":
		return "text"
	case "markdown", "md":
		return "markdown"
	case "json":
		return "json"
	default:
		return strings.ToLower(strings.TrimSpace(kind))
	}
}

func validateSteps(steps []Step) error {
	seen := make(map[string]struct{}, len(steps))
	for i, step := range steps {
		id := strings.TrimSpace(step.ID)
		if id == "" {
			return fmt.Errorf("step %d id is required", i+1)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate step id %q", id)
		}
		seen[id] = struct{}{}
	}
	for _, step := range steps {
		if step.Execution != nil {
			switch step.Execution.Kind {
			case "agent", "manual", "external":
			default:
				return fmt.Errorf("step %q has unsupported execution kind %q", step.ID, step.Execution.Kind)
			}
		}
		if step.Artifact != nil {
			switch step.Artifact.ContentKind {
			case "", "text", "markdown", "json":
			default:
				return fmt.Errorf("step %q has unsupported artifact content kind %q", step.ID, step.Artifact.ContentKind)
			}
		}
		for _, dep := range step.DependsOn {
			id := strings.TrimSpace(dep)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; !ok {
				return fmt.Errorf("step %q depends on unknown step %q", step.ID, id)
			}
		}
	}
	return nil
}

func Render(raw []byte, ctx RenderContext) RenderResult {
	var result RenderResult
	var s Schema
	if err := json.Unmarshal(raw, &s); err != nil {
		result.Warnings = append(result.Warnings, "schema could not be parsed")
		return result
	}

	markdown := strings.TrimSpace(s.Source.BodyTemplate)
	if markdown == "" && len(s.Steps) > 0 {
		var b strings.Builder
		if strings.TrimSpace(s.Name) != "" {
			fmt.Fprintf(&b, "## %s\n\n", strings.TrimSpace(s.Name))
		} else {
			b.WriteString("## Workflow\n\n")
		}
		for i, step := range s.Steps {
			title := strings.TrimSpace(step.Title)
			if title == "" {
				title = strings.TrimSpace(step.Name)
			}
			if title == "" {
				title = step.ID
			}
			fmt.Fprintf(&b, "%d. **%s**\n", i+1, title)
			desc := strings.TrimSpace(step.Description)
			if desc == "" {
				desc = strings.TrimSpace(step.BodyTemplate)
			}
			if desc != "" {
				fmt.Fprintf(&b, "   - %s\n", desc)
			}
			if step.Output != nil && strings.TrimSpace(step.Output.Description) != "" {
				fmt.Fprintf(&b, "   - Done when: %s\n", strings.TrimSpace(step.Output.Description))
			}
			if step.Review != nil && step.Review.Required {
				b.WriteString("   - Gate: human review required\n")
			}
			if step.QualityGate != nil && step.QualityGate.Enabled {
				b.WriteString("   - Gate: quality check required\n")
			}
			for _, item := range step.Checklist {
				if trimmed := strings.TrimSpace(item); trimmed != "" {
					fmt.Fprintf(&b, "   - %s\n", trimmed)
				}
			}
			b.WriteString("\n")
		}
		markdown = strings.TrimSpace(b.String())
	}
	if markdown == "" {
		result.Warnings = append(result.Warnings, "workflow has no markdown body or steps")
	}

	replacer := strings.NewReplacer(
		"{{issue_id}}", ctx.IssueID,
		"{{trigger_comment_id}}", ctx.TriggerCommentID,
	)
	result.Markdown = replacer.Replace(markdown)
	if result.Markdown != "" && !strings.HasSuffix(result.Markdown, "\n") {
		result.Markdown += "\n"
	}
	return result
}

func seedFromTemplate(key string, tpl execprotocol.Template, applicability []string) Seed {
	return systemSeed(key, tpl.Name, tpl.Description, applicability, tpl.Content, templateSteps(tpl.Slug))
}

func systemSeed(key, name, description string, applicability []string, body string, steps []Step) Seed {
	s := Schema{
		SchemaVersion: 2,
		Name:          name,
		Description:   description,
		Applicability: applicability,
		Source: Source{
			Format:       "markdown",
			BodyTemplate: strings.TrimSpace(body) + "\n",
		},
		Variables: []Variable{
			{Key: "issue_id", Description: "Multica issue UUID or identifier.", Required: true},
			{Key: "trigger_comment_id", Description: "Comment UUID for comment-triggered tasks."},
		},
		Steps: steps,
	}
	raw, _ := json.Marshal(s)
	return Seed{Key: key, Name: name, Description: description, Schema: raw}
}

func templateSteps(slug string) []Step {
	switch slug {
	case execprotocol.TrellisTaskSlug:
		return []Step{
			{ID: "context", Title: "Context first", Order: 1, Description: "Load issue details, comments, and relevant project resources."},
			{ID: "trellis-gate", Title: "Trellis availability gate", Order: 2, DependsOn: []string{"context"}, Description: "Detect, continue, or explicitly bootstrap Trellis before falling back."},
			{ID: "contract", Title: "Work contract", Order: 3, DependsOn: []string{"trellis-gate"}, Description: "Write the outcome, acceptance criteria, constraints, and verification into Trellis."},
			{ID: "implement", Title: "Plan and implement", Order: 4, DependsOn: []string{"contract"}, Description: "Move in progress and execute inside Trellis task scope."},
			{ID: "check", Title: "Check and update spec", Order: 5, DependsOn: []string{"implement"}, Description: "Run Trellis checks, targeted verification, and spec updates when durable behavior changes."},
			{ID: "finish", Title: "Finish work", Order: 6, DependsOn: []string{"check"}, Description: "Run finish-work, comment the outcome, and move the issue to review."},
			{ID: "blocked", Title: "Blocked path", Order: 7, DependsOn: []string{"contract"}, Description: "If progress is impossible, mark blocked and state the exact needed input."},
		}
	default:
		return []Step{
			{ID: "context", Title: "Context first", Order: 1, Description: "Load issue details, comments, and relevant project resources."},
			{ID: "contract", Title: "Work contract", Order: 2, DependsOn: []string{"context"}, Description: "Synthesize requested outcome, acceptance criteria, constraints, and verification."},
			{ID: "plan", Title: "Plan gate", Order: 3, DependsOn: []string{"contract"}, Description: "Plan non-trivial work before editing; return a plan when that is the deliverable."},
			{ID: "execute", Title: "Execute", Order: 4, DependsOn: []string{"plan"}, Description: "Move in progress and make the smallest coherent change."},
			{ID: "verify", Title: "Verify", Order: 5, DependsOn: []string{"execute"}, Description: "Run the checks that prove the claim and record gaps."},
			{ID: "report", Title: "Report", Order: 6, DependsOn: []string{"verify"}, Description: "Post the result and move the issue to review."},
			{ID: "blocked", Title: "Blocked path", Order: 7, DependsOn: []string{"contract"}, Description: "If progress is impossible, mark blocked and state the exact needed input."},
		}
	}
}
