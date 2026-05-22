package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const defaultPlanEngineID = "grill_with_docs"

type planEngineDefinition struct {
	ID          string
	Label       string
	Description string
	Protocol    string
	Version     string
}

type PlanEngineResponse struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Default     bool   `json:"default"`
}

type ListPlanEnginesResponse struct {
	DefaultEngine string               `json:"default_engine"`
	Engines       []PlanEngineResponse `json:"engines"`
}

type ChatPlanRunResponse struct {
	ID               string          `json:"id"`
	WorkspaceID      string          `json:"workspace_id"`
	ChatSessionID    string          `json:"chat_session_id"`
	CreatorUserID    string          `json:"creator_user_id"`
	ActorType        string          `json:"actor_type"`
	ActorID          string          `json:"actor_id"`
	LeadAgentID      string          `json:"lead_agent_id"`
	PlanEngine       string          `json:"plan_engine"`
	EngineVersion    string          `json:"engine_version"`
	Status           string          `json:"status"`
	InitialMessageID *string         `json:"initial_message_id"`
	LatestMessageID  *string         `json:"latest_message_id"`
	Summary          json.RawMessage `json:"summary"`
	CreatedAt        string          `json:"created_at"`
	UpdatedAt        string          `json:"updated_at"`
}

type ChatPlanConsultationResponse struct {
	ID                string  `json:"id"`
	PlanRunID         string  `json:"plan_run_id"`
	RequesterAgentID  string  `json:"requester_agent_id"`
	TargetAgentID     string  `json:"target_agent_id"`
	RequestMessageID  *string `json:"request_message_id"`
	ResponseMessageID *string `json:"response_message_id"`
	TaskID            *string `json:"task_id"`
	Status            string  `json:"status"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

type ChatPlanTaskData struct {
	RunID           string                         `json:"run_id"`
	ActorType       string                         `json:"actor_type"`
	ActorID         string                         `json:"actor_id"`
	LeadAgentID     string                         `json:"lead_agent_id"`
	Status          string                         `json:"status"`
	TaskKind        string                         `json:"task_kind"`
	PlanEngine      ChatPlanEngineTaskData         `json:"plan_engine"`
	Summary         json.RawMessage                `json:"summary"`
	Transcript      []ChatPlanTranscriptMessage    `json:"transcript"`
	Consultations   []ChatPlanConsultationResponse `json:"consultations,omitempty"`
	Squad           *ChatPlanSquadTaskData         `json:"squad,omitempty"`
	Consultation    *ChatPlanConsultationTaskData  `json:"consultation,omitempty"`
	ProposalPath    string                         `json:"proposal_path"`
	PlanSummaryPath string                         `json:"plan_summary_path"`
}

type ChatPlanEngineTaskData struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Protocol    string `json:"protocol"`
}

type ChatPlanTranscriptMessage struct {
	ID             string  `json:"id"`
	Role           string  `json:"role"`
	Content        string  `json:"content"`
	AuthorType     string  `json:"author_type"`
	AuthorAgentID  *string `json:"author_agent_id,omitempty"`
	ConsultationID *string `json:"consultation_id,omitempty"`
	CreatedAt      string  `json:"created_at"`
}

type ChatPlanSquadTaskData struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	LeadMention string                    `json:"lead_mention"`
	Helpers     []ChatPlanSquadHelperData `json:"helpers"`
}

type ChatPlanSquadHelperData struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
	Role    string `json:"role,omitempty"`
	Mention string `json:"mention"`
}

type ChatPlanConsultationTaskData struct {
	ID                 string `json:"id"`
	RequesterAgentID   string `json:"requester_agent_id"`
	TargetAgentID      string `json:"target_agent_id"`
	RequestMessageID   string `json:"request_message_id"`
	RequestContent     string `json:"request_content"`
	LeadMention        string `json:"lead_mention"`
	TargetAgentMention string `json:"target_agent_mention"`
}

func planEngineDefinitions() []planEngineDefinition {
	defs := []planEngineDefinition{
		{
			ID:          defaultPlanEngineID,
			Label:       "Grill with docs",
			Description: "Interrogate the idea one question at a time, prefer repository and document evidence, and propose issues only after requirements are sharp.",
			Protocol: strings.TrimSpace(`You are running the Grill with docs planning protocol.

- Ask one focused question at a time when the answer materially changes the plan.
- Prefer inspecting available project resources, repository context, and chat transcript evidence before asking the user.
- Challenge vague goals, hidden constraints, unclear acceptance criteria, and missing ownership.
- Track confirmed requirements, rejected options, consensus notes, and open questions in the plan summary manifest.
- Do not produce issue proposals until the plan is specific enough for an implementer to execute without guessing.
- When ready, write reviewable issue proposals instead of creating issues directly.`),
		},
		{
			ID:          "brainstorming",
			Label:       "Brainstorming",
			Description: "Diverge across possible approaches, then converge on a practical issue split.",
			Protocol: strings.TrimSpace(`You are running the Brainstorming planning protocol.

- First expand the option space: identify plausible approaches, variants, risks, and unknowns.
- Group ideas by user value and implementation dependency.
- Converge only after comparing tradeoffs and eliminating weak options.
- Keep the plan summary current with options kept, options rejected, consensus notes, and open questions.
- When ready, propose a concise issue set that preserves optionality where useful but avoids vague catch-all tasks.`),
		},
		{
			ID:          "office_hours",
			Label:       "Office hours",
			Description: "Challenge product and strategy assumptions before turning the idea into execution work.",
			Protocol: strings.TrimSpace(`You are running the Office hours planning protocol.

- Pressure-test the problem, user, urgency, and success metric before implementation details.
- Call out weak assumptions directly and ask for the smallest missing fact that would change the plan.
- Distinguish must-have work from nice-to-have polish.
- Record product consensus, rejected bets, and unresolved risks in the plan summary manifest.
- When ready, propose issues that preserve the strategic intent and include clear acceptance criteria.`),
		},
	}
	for i := range defs {
		defs[i].Version = planEngineVersion(defs[i])
	}
	return defs
}

func planEngineVersion(def planEngineDefinition) string {
	sum := sha256.Sum256([]byte(def.ID + "\n" + def.Protocol))
	return hex.EncodeToString(sum[:])[:16]
}

func findPlanEngine(id string) (planEngineDefinition, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		id = defaultPlanEngineID
	}
	for _, def := range planEngineDefinitions() {
		if def.ID == id {
			return def, true
		}
	}
	return planEngineDefinition{}, false
}

func planEngineToResponse(def planEngineDefinition) PlanEngineResponse {
	return PlanEngineResponse{
		ID:          def.ID,
		Label:       def.Label,
		Description: def.Description,
		Version:     def.Version,
		Default:     def.ID == defaultPlanEngineID,
	}
}

func planEngineToTaskData(def planEngineDefinition) ChatPlanEngineTaskData {
	return ChatPlanEngineTaskData{
		ID:          def.ID,
		Label:       def.Label,
		Description: def.Description,
		Version:     def.Version,
		Protocol:    def.Protocol,
	}
}

func (h *Handler) ListChatPlanEngines(w http.ResponseWriter, r *http.Request) {
	defs := planEngineDefinitions()
	resp := ListPlanEnginesResponse{
		DefaultEngine: defaultPlanEngineID,
		Engines:       make([]PlanEngineResponse, 0, len(defs)),
	}
	for _, def := range defs {
		resp.Engines = append(resp.Engines, planEngineToResponse(def))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListChatPlanRuns(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")
	session, ok := h.gateChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return
	}
	rows, err := h.Queries.ListChatPlanRunsBySession(r.Context(), db.ListChatPlanRunsBySessionParams{
		ChatSessionID: session.ID,
		WorkspaceID:   session.WorkspaceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list plan runs")
		return
	}
	out := make([]ChatPlanRunResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, chatPlanRunToResponse(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan_runs": out, "total": len(out)})
}

func (h *Handler) CancelChatPlanRun(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	planRunID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "planRunId"), "plan run id")
	if !ok {
		return
	}
	run, err := h.Queries.GetChatPlanRun(r.Context(), planRunID)
	if err != nil {
		writeError(w, http.StatusNotFound, "plan run not found")
		return
	}
	if _, ok := h.gateChatSessionForUser(w, r, userID, workspaceID, uuidToString(run.ChatSessionID)); !ok {
		return
	}
	updated, err := h.Queries.CancelChatPlanRun(r.Context(), db.CancelChatPlanRunParams{
		ID:          planRunID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusBadRequest, "plan run is not active")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to cancel plan run")
		return
	}
	h.publishChatPlanRunUpdate(uuidToString(updated.WorkspaceID), updated.ChatSessionID, updated.ID, "member", userID)
	writeJSON(w, http.StatusOK, chatPlanRunToResponse(updated))
}

func (h *Handler) publishChatPlanRunUpdate(workspaceID string, sessionID, planRunID pgtype.UUID, actorType, actorID string) {
	if workspaceID == "" || !sessionID.Valid {
		return
	}
	sessionIDStr := uuidToString(sessionID)
	h.publishChat(protocol.EventChatPlanRunsUpdated, workspaceID, actorType, actorID, sessionIDStr, protocol.ChatPlanRunsUpdatedPayload{
		ChatSessionID: sessionIDStr,
		PlanRunID:     uuidToString(planRunID),
	})
}

func (h *Handler) resolvePlanActorForSend(w http.ResponseWriter, r *http.Request, session db.ChatSession, actorTypeRaw, actorIDRaw string) (actorType string, actorID pgtype.UUID, leadAgentID pgtype.UUID, ok bool) {
	actorType = strings.TrimSpace(actorTypeRaw)
	if actorType == "" {
		actorType = "agent"
	}
	switch actorType {
	case "agent":
		if strings.TrimSpace(actorIDRaw) == "" {
			actorID = session.AgentID
		} else {
			parsed, parsedOK := parseUUIDOrBadRequest(w, actorIDRaw, "plan_actor_id")
			if !parsedOK {
				return "", pgtype.UUID{}, pgtype.UUID{}, false
			}
			actorID = parsed
		}
		if uuidToString(actorID) != uuidToString(session.AgentID) {
			writeError(w, http.StatusBadRequest, "agent plan actor must match the chat session agent")
			return "", pgtype.UUID{}, pgtype.UUID{}, false
		}
		leadAgentID = actorID
		return actorType, actorID, leadAgentID, true
	case "squad":
		if strings.TrimSpace(actorIDRaw) == "" {
			writeError(w, http.StatusBadRequest, "plan_actor_id is required for squad plan actor")
			return "", pgtype.UUID{}, pgtype.UUID{}, false
		}
		squadID, parsedOK := parseUUIDOrBadRequest(w, actorIDRaw, "plan_actor_id")
		if !parsedOK {
			return "", pgtype.UUID{}, pgtype.UUID{}, false
		}
		squad, err := h.Queries.GetSquadInWorkspace(r.Context(), db.GetSquadInWorkspaceParams{
			ID:          squadID,
			WorkspaceID: session.WorkspaceID,
		})
		if err != nil || squad.ArchivedAt.Valid {
			writeError(w, http.StatusNotFound, "squad not found")
			return "", pgtype.UUID{}, pgtype.UUID{}, false
		}
		return actorType, squad.ID, squad.LeaderID, true
	default:
		writeError(w, http.StatusBadRequest, "plan_actor_type must be agent or squad")
		return "", pgtype.UUID{}, pgtype.UUID{}, false
	}
}

func chatPlanRunToResponse(row db.ChatPlanRun) ChatPlanRunResponse {
	return ChatPlanRunResponse{
		ID:               uuidToString(row.ID),
		WorkspaceID:      uuidToString(row.WorkspaceID),
		ChatSessionID:    uuidToString(row.ChatSessionID),
		CreatorUserID:    uuidToString(row.CreatorUserID),
		ActorType:        row.ActorType,
		ActorID:          uuidToString(row.ActorID),
		LeadAgentID:      uuidToString(row.LeadAgentID),
		PlanEngine:       row.PlanEngine,
		EngineVersion:    row.EngineVersion,
		Status:           row.Status,
		InitialMessageID: uuidToPtr(row.InitialMessageID),
		LatestMessageID:  uuidToPtr(row.LatestMessageID),
		Summary:          planJSONRawOrEmpty(row.Summary),
		CreatedAt:        timestampToString(row.CreatedAt),
		UpdatedAt:        timestampToString(row.UpdatedAt),
	}
}

func chatPlanConsultationToResponse(row db.ChatPlanConsultation) ChatPlanConsultationResponse {
	return ChatPlanConsultationResponse{
		ID:                uuidToString(row.ID),
		PlanRunID:         uuidToString(row.PlanRunID),
		RequesterAgentID:  uuidToString(row.RequesterAgentID),
		TargetAgentID:     uuidToString(row.TargetAgentID),
		RequestMessageID:  uuidToPtr(row.RequestMessageID),
		ResponseMessageID: uuidToPtr(row.ResponseMessageID),
		TaskID:            uuidToPtr(row.TaskID),
		Status:            row.Status,
		CreatedAt:         timestampToString(row.CreatedAt),
		UpdatedAt:         timestampToString(row.UpdatedAt),
	}
}

func planJSONRawOrEmpty(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	return json.RawMessage(raw)
}

func (h *Handler) populatePlanClaimContext(ctx context.Context, resp *AgentTaskResponse, task db.AgentTaskQueue, session db.ChatSession) {
	if !task.ChatPlanRunID.Valid {
		return
	}
	run, err := h.Queries.GetChatPlanRun(ctx, task.ChatPlanRunID)
	if err != nil {
		return
	}
	engine, ok := findPlanEngine(run.PlanEngine)
	if !ok {
		engine = planEngineDefinition{
			ID:          run.PlanEngine,
			Label:       run.PlanEngine,
			Description: "Unknown plan engine.",
			Protocol:    "Continue the chat plan run using the existing transcript and plan summary.",
			Version:     run.EngineVersion,
		}
	}
	engineData := planEngineToTaskData(engine)
	engineData.Version = run.EngineVersion
	taskKind := task.ChatTaskKind
	if taskKind == "" {
		taskKind = service.ChatTaskKindPlanLead
	}
	plan := &ChatPlanTaskData{
		RunID:           uuidToString(run.ID),
		ActorType:       run.ActorType,
		ActorID:         uuidToString(run.ActorID),
		LeadAgentID:     uuidToString(run.LeadAgentID),
		Status:          run.Status,
		TaskKind:        taskKind,
		PlanEngine:      engineData,
		Summary:         planJSONRawOrEmpty(run.Summary),
		ProposalPath:    ".multica/chats/" + uuidToString(session.ID) + "/issue-proposals.json",
		PlanSummaryPath: ".multica/chats/" + uuidToString(session.ID) + "/plan-summary.json",
	}
	plan.Transcript = h.planTranscript(ctx, run.ID, session.ID)
	if consults, err := h.Queries.ListChatPlanConsultationsByRun(ctx, run.ID); err == nil {
		plan.Consultations = make([]ChatPlanConsultationResponse, 0, len(consults))
		for _, row := range consults {
			plan.Consultations = append(plan.Consultations, chatPlanConsultationToResponse(row))
		}
	}
	if run.ActorType == "squad" && run.ActorID.Valid {
		plan.Squad = h.planSquadTaskData(ctx, run)
	}
	if task.ChatPlanConsultationID.Valid {
		plan.Consultation = h.planConsultationTaskData(ctx, task.ChatPlanConsultationID, run, plan.Squad)
	}
	resp.Plan = plan
}

func (h *Handler) planTranscript(ctx context.Context, planRunID, sessionID pgtype.UUID) []ChatPlanTranscriptMessage {
	msgs, err := h.Queries.ListChatMessages(ctx, sessionID)
	if err != nil {
		return nil
	}
	out := make([]ChatPlanTranscriptMessage, 0, len(msgs))
	for _, msg := range msgs {
		if msg.PlanRunID.Valid && util.UUIDToString(msg.PlanRunID) != util.UUIDToString(planRunID) {
			continue
		}
		if !msg.PlanRunID.Valid && msg.ConsultationID.Valid {
			continue
		}
		out = append(out, ChatPlanTranscriptMessage{
			ID:             uuidToString(msg.ID),
			Role:           msg.Role,
			Content:        msg.Content,
			AuthorType:     msg.AuthorType,
			AuthorAgentID:  uuidToPtr(msg.AuthorAgentID),
			ConsultationID: uuidToPtr(msg.ConsultationID),
			CreatedAt:      timestampToString(msg.CreatedAt),
		})
	}
	return out
}

func (h *Handler) planSquadTaskData(ctx context.Context, run db.ChatPlanRun) *ChatPlanSquadTaskData {
	squad, err := h.Queries.GetSquad(ctx, run.ActorID)
	if err != nil {
		return nil
	}
	leadName := "Lead"
	if lead, err := h.Queries.GetAgent(ctx, run.LeadAgentID); err == nil {
		leadName = lead.Name
	}
	out := &ChatPlanSquadTaskData{
		ID:          uuidToString(squad.ID),
		Name:        squad.Name,
		LeadMention: formatMention(leadName, "agent", uuidToString(run.LeadAgentID)),
		Helpers:     []ChatPlanSquadHelperData{},
	}
	members, err := h.Queries.ListSquadMembers(ctx, squad.ID)
	if err != nil {
		return out
	}
	leadID := uuidToString(run.LeadAgentID)
	for _, member := range members {
		if member.MemberType != "agent" || uuidToString(member.MemberID) == leadID {
			continue
		}
		agent, err := h.Queries.GetAgent(ctx, member.MemberID)
		if err != nil || agent.ArchivedAt.Valid {
			continue
		}
		out.Helpers = append(out.Helpers, ChatPlanSquadHelperData{
			AgentID: uuidToString(agent.ID),
			Name:    agent.Name,
			Role:    strings.TrimSpace(member.Role),
			Mention: formatMention(agent.Name, "agent", uuidToString(agent.ID)),
		})
	}
	return out
}

func (h *Handler) planConsultationTaskData(ctx context.Context, consultationID pgtype.UUID, run db.ChatPlanRun, squad *ChatPlanSquadTaskData) *ChatPlanConsultationTaskData {
	consultation, err := h.Queries.GetChatPlanConsultation(ctx, consultationID)
	if err != nil {
		return nil
	}
	requestContent := ""
	if consultation.RequestMessageID.Valid {
		if msg, err := h.Queries.GetChatMessage(ctx, consultation.RequestMessageID); err == nil {
			requestContent = msg.Content
		}
	}
	leadMention := ""
	if squad != nil {
		leadMention = squad.LeadMention
	}
	targetMention := ""
	if target, err := h.Queries.GetAgent(ctx, consultation.TargetAgentID); err == nil {
		targetMention = formatMention(target.Name, "agent", uuidToString(target.ID))
	}
	return &ChatPlanConsultationTaskData{
		ID:                 uuidToString(consultation.ID),
		RequesterAgentID:   uuidToString(consultation.RequesterAgentID),
		TargetAgentID:      uuidToString(consultation.TargetAgentID),
		RequestMessageID:   uuidToString(consultation.RequestMessageID),
		RequestContent:     requestContent,
		LeadMention:        leadMention,
		TargetAgentMention: targetMention,
	}
}
