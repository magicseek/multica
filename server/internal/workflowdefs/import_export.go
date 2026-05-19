package workflowdefs

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v2"
)

type ImportResult struct {
	Schema   []byte
	Warnings []string
}

type ExportResult struct {
	Format   string   `json:"format"`
	Content  string   `json:"content"`
	Warnings []string `json:"warnings,omitempty"`
}

func ImportSchema(format string, content []byte, fallbackName, fallbackDescription string) (ImportResult, error) {
	doc, err := decodeImportDocument(format, content)
	if err != nil {
		return ImportResult{}, err
	}
	doc = unwrapWorkflowDocument(doc)

	warnings := skippedAIDeskFieldWarnings(doc)
	var schemaDoc map[string]any
	if looksLikeAIDeskWorkflow(doc) {
		var convertWarnings []string
		schemaDoc, convertWarnings = convertAIDeskWorkflow(doc, fallbackName, fallbackDescription)
		warnings = append(warnings, convertWarnings...)
	} else {
		schemaDoc = doc
	}

	raw, err := json.Marshal(schemaDoc)
	if err != nil {
		return ImportResult{}, fmt.Errorf("marshal imported workflow schema: %w", err)
	}
	normalized, err := NormalizeSchema(raw, fallbackName, fallbackDescription)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Schema: normalized, Warnings: uniqueWarnings(warnings)}, nil
}

func ExportSchema(raw []byte, format string) (ExportResult, error) {
	normalized, err := NormalizeSchema(raw, "", "")
	if err != nil {
		return ExportResult{}, err
	}
	format = normalizeImportFormat(format)
	var payload any
	if err := json.Unmarshal(normalized, &payload); err != nil {
		return ExportResult{}, fmt.Errorf("parse workflow schema for export: %w", err)
	}

	switch format {
	case "json":
		content, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return ExportResult{}, fmt.Errorf("export workflow json: %w", err)
		}
		return ExportResult{Format: "json", Content: string(content) + "\n"}, nil
	case "yaml":
		content, err := yaml.Marshal(payload)
		if err != nil {
			return ExportResult{}, fmt.Errorf("export workflow yaml: %w", err)
		}
		return ExportResult{Format: "yaml", Content: string(content)}, nil
	default:
		return ExportResult{}, fmt.Errorf("unsupported export format %q", format)
	}
}

func SchemaMetadata(raw []byte) (name, description string) {
	var schema Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		return "", ""
	}
	return strings.TrimSpace(schema.Name), strings.TrimSpace(schema.Description)
}

func decodeImportDocument(format string, content []byte) (map[string]any, error) {
	if len(strings.TrimSpace(string(content))) == 0 {
		return nil, fmt.Errorf("workflow import content is required")
	}

	format = normalizeImportFormat(format)
	var decoded any
	switch format {
	case "json":
		if err := json.Unmarshal(content, &decoded); err != nil {
			return nil, fmt.Errorf("parse workflow json: %w", err)
		}
	case "yaml":
		if err := yaml.Unmarshal(content, &decoded); err != nil {
			return nil, fmt.Errorf("parse workflow yaml: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported import format %q", format)
	}
	converted := normalizeYAMLValue(decoded)
	doc, ok := converted.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("workflow import must be an object")
	}
	return doc, nil
}

func normalizeImportFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "yml", "yaml":
		return "yaml"
	case "json":
		return "json"
	default:
		return strings.ToLower(strings.TrimSpace(format))
	}
}

func normalizeYAMLValue(v any) any {
	switch value := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))
		for k, item := range value {
			out[k] = normalizeYAMLValue(item)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[string]any, len(value))
		for k, item := range value {
			out[fmt.Sprint(k)] = normalizeYAMLValue(item)
		}
		return out
	case []interface{}:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = normalizeYAMLValue(item)
		}
		return out
	default:
		return value
	}
}

func unwrapWorkflowDocument(doc map[string]any) map[string]any {
	if schema := mapValue(doc["schema"]); looksLikeWorkflowSchema(schema) {
		return schema
	}
	if workflow := mapValue(doc["workflow"]); len(workflow) > 0 {
		return workflow
	}
	return doc
}

func looksLikeWorkflowSchema(doc map[string]any) bool {
	if len(doc) == 0 {
		return false
	}
	_, hasSchemaVersion := doc["schema_version"]
	_, hasApplicability := doc["applicability"]
	_, hasSteps := doc["steps"]
	_, hasSource := doc["source"]
	return hasSchemaVersion || hasApplicability || hasSteps || hasSource
}

func looksLikeAIDeskWorkflow(doc map[string]any) bool {
	if _, ok := doc["step_definitions"]; ok {
		return true
	}
	if _, ok := doc["workflow_type"]; ok {
		return true
	}
	if source, ok := doc["source"].(string); ok && strings.TrimSpace(source) != "" {
		return true
	}
	for _, item := range sliceValue(firstValue(doc, "steps", "step_definitions")) {
		step := mapValue(item)
		for _, key := range []string{"agent_prompt", "rules", "review_required", "quality_gate_prompt", "quality_report_mode", "artifact_template_content", "default_execution_mode"} {
			if _, ok := step[key]; ok {
				return true
			}
		}
	}
	return false
}

func skippedAIDeskFieldWarnings(doc map[string]any) []string {
	var warnings []string
	for _, key := range []string{"party_mode", "party_mode_discuss", "party_mode_review", "pre_added_agents", "participants", "acl", "ticket_required"} {
		if _, ok := doc[key]; ok {
			warnings = append(warnings, fmt.Sprintf("ai-desk field %q was intentionally skipped", key))
		}
	}
	if _, ok := doc["engine"]; ok {
		warnings = append(warnings, "ai-desk engine label was intentionally skipped")
	}
	if _, ok := doc["workflow_engine"]; ok {
		warnings = append(warnings, "ai-desk workflow engine label was intentionally skipped")
	}
	if workflowType := strings.ToUpper(stringValue(doc["workflow_type"])); workflowType != "" {
		if workflowType == "AUTOMATION" {
			warnings = append(warnings, "ai-desk AUTOMATION workflow type maps to Multica Autopilot and was not imported as a workflow type")
		} else {
			warnings = append(warnings, "ai-desk workflow_type was intentionally skipped")
		}
	}
	if source, ok := doc["source"].(string); ok && strings.TrimSpace(source) != "" {
		warnings = append(warnings, "ai-desk source visibility was not migrated; imported workflow uses workspace permissions")
	}
	return warnings
}

func convertAIDeskWorkflow(doc map[string]any, fallbackName, fallbackDescription string) (map[string]any, []string) {
	var warnings []string
	name := firstString(doc, "name", "title")
	if name == "" {
		name = fallbackName
	}
	description := firstString(doc, "description", "summary")
	if description == "" {
		description = fallbackDescription
	}

	schema := map[string]any{
		"schema_version": 2,
		"name":           name,
		"description":    description,
		"applicability":  applicabilityFromAIDesk(doc),
		"source": map[string]any{
			"format":        "markdown",
			"body_template": firstString(doc, "body_template", "prompt", "markdown"),
		},
		"steps": []any{},
		"gates": []any{},
	}
	if category := firstString(doc, "category"); category != "" {
		schema["category"] = category
	}

	rawSteps := sliceValue(firstValue(doc, "step_definitions", "steps"))
	steps := make([]any, 0, len(rawSteps))
	stepIDs := make(map[string]struct{}, len(rawSteps))
	for i, item := range rawSteps {
		stepDoc := mapValue(item)
		step := convertAIDeskStep(stepDoc, i, &warnings)
		if id := stringValue(step["id"]); id != "" {
			stepIDs[id] = struct{}{}
		}
		steps = append(steps, step)
	}
	for _, stepAny := range steps {
		step := mapValue(stepAny)
		artifact := mapValue(step["artifact"])
		for _, inputAny := range sliceValue(artifact["inputs"]) {
			input := mapValue(inputAny)
			if stepID := stringValue(input["step_id"]); stepID != "" {
				if _, ok := stepIDs[stepID]; !ok {
					warnings = append(warnings, fmt.Sprintf("step %q references unknown artifact input step %q", stringValue(step["id"]), stepID))
				}
			}
		}
	}
	schema["steps"] = steps
	return schema, warnings
}

func applicabilityFromAIDesk(doc map[string]any) []string {
	if items := stringSliceValue(doc["applicability"]); len(items) > 0 {
		return items
	}
	if strings.EqualFold(stringValue(doc["ticket_required"]), "false") {
		return []string{"chat", "assignment"}
	}
	return []string{"assignment"}
}

func convertAIDeskStep(stepDoc map[string]any, index int, warnings *[]string) map[string]any {
	title := firstString(stepDoc, "title", "name")
	id := firstString(stepDoc, "id", "key", "slug")
	if id == "" {
		id = slugify(title)
	}
	if id == "" {
		id = fmt.Sprintf("step-%d", index+1)
	}
	if title == "" {
		title = id
	}

	step := map[string]any{
		"id":         id,
		"title":      title,
		"order":      intValue(firstValue(stepDoc, "order", "order_index"), index+1),
		"depends_on": stringSliceValue(firstValue(stepDoc, "depends_on", "depends_on_steps")),
		"execution": map[string]any{
			"kind":   executionKindFromAIDesk(firstValue(stepDoc, "execution_mode", "default_execution_mode")),
			"prompt": firstString(stepDoc, "agent_prompt", "prompt"),
			"rules":  firstString(stepDoc, "rules"),
		},
	}
	if description := firstString(stepDoc, "description"); description != "" {
		step["description"] = description
	}
	if body := firstString(stepDoc, "body_template"); body != "" {
		step["body_template"] = body
	}
	if checklist := stringSliceValue(stepDoc["checklist"]); len(checklist) > 0 {
		step["checklist"] = checklist
	}
	if required, ok := boolValue(stepDoc["required"]); ok {
		step["required"] = required
	}

	artifact := convertAIDeskArtifact(stepDoc, warnings, id)
	if len(artifact) > 0 {
		step["artifact"] = artifact
	}
	if reviewRequired, ok := boolValue(firstValue(stepDoc, "review_required")); ok {
		step["review"] = map[string]any{"required": reviewRequired}
	}
	qualityGate := convertAIDeskQualityGate(stepDoc)
	if len(qualityGate) > 0 {
		step["quality_gate"] = qualityGate
	}
	for _, key := range []string{"party_mode_discuss", "party_mode_review", "pre_added_agents", "participants"} {
		if _, ok := stepDoc[key]; ok {
			*warnings = append(*warnings, fmt.Sprintf("step %q ai-desk field %q was intentionally skipped", id, key))
		}
	}
	return step
}

func convertAIDeskArtifact(stepDoc map[string]any, warnings *[]string, stepID string) map[string]any {
	artifactDoc := mapValue(stepDoc["artifact"])
	name := firstString(stepDoc, "artifact_name")
	if name == "" {
		name = stringValue(artifactDoc["name"])
	}
	contentKind := firstString(stepDoc, "artifact_content_kind", "content_kind")
	templateContent := firstString(stepDoc, "artifact_template_content")
	templateDoc := mapValue(artifactDoc["template"])
	if templateContent == "" {
		templateContent = stringValue(templateDoc["content"])
	}
	inputs := artifactInputsFromValue(firstValue(stepDoc, "input_artifacts", "artifact_inputs"))
	if len(inputs) == 0 {
		inputs = artifactInputsFromValue(artifactDoc["inputs"])
	}

	artifact := map[string]any{}
	if name != "" {
		artifact["name"] = name
	}
	if contentKind != "" {
		artifact["content_kind"] = contentKind
	}
	if templateContent != "" {
		format := firstString(stepDoc, "artifact_template_format")
		if format == "" {
			format = stringValue(templateDoc["format"])
		}
		if format == "" {
			format = "markdown"
		}
		artifact["template"] = map[string]any{
			"format":  format,
			"content": templateContent,
			"files":   artifactTemplateFilesFromValue(firstValue(stepDoc, "artifact_template_files", "template_files")),
		}
	}
	if len(inputs) > 0 {
		artifact["inputs"] = inputs
	}
	if templateID := firstString(stepDoc, "artifact_template_id"); templateID != "" && templateContent == "" {
		*warnings = append(*warnings, fmt.Sprintf("step %q references artifact_template_id %q; external template content was not available", stepID, templateID))
	}
	return artifact
}

func convertAIDeskQualityGate(stepDoc map[string]any) map[string]any {
	qualityDoc := mapValue(stepDoc["quality_gate"])
	prompt := firstString(stepDoc, "quality_gate_prompt")
	if prompt == "" {
		prompt = stringValue(qualityDoc["prompt"])
	}
	reportMode := firstString(stepDoc, "quality_report_mode")
	if reportMode == "" {
		reportMode = stringValue(qualityDoc["report_mode"])
	}
	blocking, blockingOK := boolValue(firstValue(stepDoc, "quality_gate_blocking", "blocking_quality_gate"))
	if !blockingOK {
		blocking, blockingOK = boolValue(qualityDoc["blocking"])
	}
	enabled, enabledOK := boolValue(qualityDoc["enabled"])

	gate := map[string]any{}
	if prompt != "" {
		gate["prompt"] = prompt
		gate["enabled"] = true
	}
	if reportMode != "" {
		gate["report_mode"] = reportMode
	}
	if blockingOK {
		gate["blocking"] = blocking
	}
	if enabledOK {
		gate["enabled"] = enabled
	}
	return gate
}

func executionKindFromAIDesk(v any) string {
	switch strings.ToUpper(strings.TrimSpace(stringValue(v))) {
	case "MANUAL":
		return "manual"
	case "EXTERNAL", "EXTERNAL_AGENT":
		return "external"
	default:
		return "agent"
	}
}

func artifactInputsFromValue(v any) []map[string]any {
	items := sliceValue(v)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		switch value := item.(type) {
		case string:
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				out = append(out, map[string]any{"artifact_name": trimmed})
			}
		default:
			input := mapValue(value)
			if len(input) == 0 {
				continue
			}
			normalized := map[string]any{}
			if stepID := firstString(input, "step_id", "step", "producer_step_id"); stepID != "" {
				normalized["step_id"] = stepID
			}
			if name := firstString(input, "artifact_name", "name"); name != "" {
				normalized["artifact_name"] = name
			}
			if alias := firstString(input, "alias", "input_name"); alias != "" {
				normalized["name"] = alias
			}
			if required, ok := boolValue(input["required"]); ok {
				normalized["required"] = required
			}
			if len(normalized) > 0 {
				out = append(out, normalized)
			}
		}
	}
	return out
}

func artifactTemplateFilesFromValue(v any) []map[string]any {
	items := sliceValue(v)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		file := mapValue(item)
		if len(file) == 0 {
			continue
		}
		normalized := map[string]any{}
		if path := firstString(file, "path", "name"); path != "" {
			normalized["path"] = path
		}
		if content := firstString(file, "content"); content != "" {
			normalized["content"] = content
		}
		if len(normalized) > 0 {
			out = append(out, normalized)
		}
	}
	return out
}

func uniqueWarnings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func firstValue(doc map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := doc[key]; ok {
			return value
		}
	}
	return nil
}

func firstString(doc map[string]any, keys ...string) string {
	return strings.TrimSpace(stringValue(firstValue(doc, keys...)))
}

func stringValue(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	default:
		return fmt.Sprint(value)
	}
}

func intValue(v any, fallback int) int {
	switch value := v.(type) {
	case int:
		if value > 0 {
			return value
		}
	case int64:
		if value > 0 {
			return int(value)
		}
	case float64:
		if value > 0 {
			return int(value)
		}
	case json.Number:
		if n, err := value.Int64(); err == nil && n > 0 {
			return int(n)
		}
	}
	return fallback
}

func boolValue(v any) (bool, bool) {
	switch value := v.(type) {
	case bool:
		return value, true
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "yes", "1":
			return true, true
		case "false", "no", "0":
			return false, true
		}
	}
	return false, false
}

func stringSliceValue(v any) []string {
	switch value := v.(type) {
	case []string:
		return normalizedStrings(value)
	case string:
		return normalizedStrings(strings.Split(value, ","))
	default:
		items := sliceValue(value)
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s := strings.TrimSpace(stringValue(item)); s != "" {
				out = append(out, s)
			}
		}
		return normalizedStrings(out)
	}
}

func mapValue(v any) map[string]any {
	switch value := v.(type) {
	case map[string]any:
		return value
	default:
		converted, ok := normalizeYAMLValue(value).(map[string]any)
		if !ok {
			return nil
		}
		return converted
	}
}

func sliceValue(v any) []any {
	switch value := v.(type) {
	case []any:
		return value
	case []string:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = item
		}
		return out
	default:
		converted, ok := normalizeYAMLValue(value).([]any)
		if !ok {
			return nil
		}
		return converted
	}
}

var slugInvalidChars = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = slugInvalidChars.ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}
