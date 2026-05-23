package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	chatRoutingSourceExplicitMention = "explicit_mention"
	chatRoutingSourceContinuation    = "continuation"
	chatRoutingSourceDefault         = "default"

	chatRoutingStatusRouted  = "routed"
	chatRoutingStatusBlocked = "blocked"
)

type chatRoutingTarget struct {
	RecipientType   string
	RecipientID     pgtype.UUID
	ResolvedAgentID pgtype.UUID
	Source          string
	Status          string
	WarningCode     string
	WarningMessage  string
}

type chatRoutingDecision struct {
	Targets     []chatRoutingTarget
	NeedsTarget *ChatNeedsTargetResponse
}

type ChatNeedsTargetResponse struct {
	Error      string                             `json:"error"`
	Code       string                             `json:"code"`
	Candidates []ChatNeedsTargetCandidateResponse `json:"candidates"`
}

type ChatNeedsTargetCandidateResponse struct {
	RecipientType   string  `json:"recipient_type"`
	RecipientID     string  `json:"recipient_id"`
	ResolvedAgentID *string `json:"resolved_agent_id,omitempty"`
}

type chatDirectedStateCandidate struct {
	RecipientType   string  `json:"recipient_type"`
	RecipientID     string  `json:"recipient_id"`
	ResolvedAgentID *string `json:"resolved_agent_id,omitempty"`
}

func (h *Handler) resolveMemberChatRouting(ctx context.Context, r *http.Request, userID, workspaceID string, session db.ChatSession, content string, isPlanMode bool, planRun db.ChatPlanRun, planRunActive bool, planActorType string, planActorID, leadAgentID pgtype.UUID) (chatRoutingDecision, error) {
	if isPlanMode {
		source := chatRoutingSourceDefault
		actorType := planActorType
		actorID := planActorID
		resolvedLeadID := leadAgentID
		if planRunActive {
			source = chatRoutingSourceContinuation
			actorType = planRun.ActorType
			actorID = planRun.ActorID
			resolvedLeadID = planRun.LeadAgentID
		}
		if actorType == "squad" && actorID.Valid {
			return chatRoutingDecision{Targets: []chatRoutingTarget{{
				RecipientType:   "squad",
				RecipientID:     actorID,
				ResolvedAgentID: resolvedLeadID,
				Source:          source,
				Status:          chatRoutingStatusRouted,
			}}}, nil
		}
		return chatRoutingDecision{Targets: []chatRoutingTarget{{
			RecipientType:   "agent",
			RecipientID:     resolvedLeadID,
			ResolvedAgentID: resolvedLeadID,
			Source:          source,
			Status:          chatRoutingStatusRouted,
		}}}, nil
	}

	mentions := util.ParseMentions(content)
	targets := make([]chatRoutingTarget, 0, len(mentions))
	for _, mention := range mentions {
		if mention.Type != "agent" && mention.Type != "squad" {
			continue
		}
		recipientID, err := util.ParseUUID(mention.ID)
		if err != nil {
			continue
		}
		targets = append(targets, h.resolveChatRoutingTarget(ctx, r, userID, workspaceID, session.WorkspaceID, mention.Type, recipientID, chatRoutingSourceExplicitMention))
	}
	if len(targets) > 0 {
		return chatRoutingDecision{Targets: targets}, nil
	}

	state, err := h.Queries.GetChatSessionDirectedState(ctx, session.ID)
	if err == nil {
		if state.State == "ambiguous" {
			return chatRoutingDecision{NeedsTarget: chatNeedsTargetResponse(state)}, nil
		}
		if state.State == "active" && state.ActiveRecipientType.Valid && state.ActiveRecipientID.Valid {
			target := h.resolveChatRoutingTarget(ctx, r, userID, workspaceID, session.WorkspaceID, state.ActiveRecipientType.String, state.ActiveRecipientID, chatRoutingSourceContinuation)
			return chatRoutingDecision{Targets: []chatRoutingTarget{target}}, nil
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return chatRoutingDecision{}, err
	}

	return chatRoutingDecision{Targets: []chatRoutingTarget{{
		RecipientType:   "agent",
		RecipientID:     session.AgentID,
		ResolvedAgentID: session.AgentID,
		Source:          chatRoutingSourceDefault,
		Status:          chatRoutingStatusRouted,
	}}}, nil
}

func (h *Handler) resolveChatRoutingTarget(ctx context.Context, r *http.Request, userID, workspaceID string, workspaceUUID pgtype.UUID, recipientType string, recipientID pgtype.UUID, source string) chatRoutingTarget {
	target := chatRoutingTarget{
		RecipientType: recipientType,
		RecipientID:   recipientID,
		Source:        source,
		Status:        chatRoutingStatusRouted,
	}
	switch recipientType {
	case "agent":
		agent, err := h.Queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{
			ID:          recipientID,
			WorkspaceID: workspaceUUID,
		})
		if err != nil {
			target.Status = chatRoutingStatusBlocked
			target.WarningCode = "recipient_not_found"
			target.WarningMessage = "Mentioned agent was not found in this workspace."
			return target
		}
		if agent.ArchivedAt.Valid {
			target.Status = chatRoutingStatusBlocked
			target.WarningCode = "recipient_archived"
			target.WarningMessage = "Mentioned agent is archived."
			return target
		}
		actorType, actorID := h.resolveActor(r, userID, workspaceID)
		if !h.canAccessPrivateAgent(ctx, agent, actorType, actorID, workspaceID) {
			target.Status = chatRoutingStatusBlocked
			target.WarningCode = "recipient_forbidden"
			target.WarningMessage = "You do not have access to the mentioned agent."
			return target
		}
		target.ResolvedAgentID = recipientID
		return target
	case "squad":
		squad, err := h.Queries.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{
			ID:          recipientID,
			WorkspaceID: workspaceUUID,
		})
		if err != nil {
			target.Status = chatRoutingStatusBlocked
			target.WarningCode = "recipient_not_found"
			target.WarningMessage = "Mentioned squad was not found in this workspace."
			return target
		}
		if squad.ArchivedAt.Valid {
			target.Status = chatRoutingStatusBlocked
			target.WarningCode = "recipient_archived"
			target.WarningMessage = "Mentioned squad is archived."
			return target
		}
		leader, err := h.Queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{
			ID:          squad.LeaderID,
			WorkspaceID: workspaceUUID,
		})
		if err != nil || leader.ArchivedAt.Valid {
			target.Status = chatRoutingStatusBlocked
			target.WarningCode = "squad_lead_unavailable"
			target.WarningMessage = "Mentioned squad has no available lead agent."
			return target
		}
		actorType, actorID := h.resolveActor(r, userID, workspaceID)
		if !h.canAccessPrivateAgent(ctx, leader, actorType, actorID, workspaceID) {
			target.Status = chatRoutingStatusBlocked
			target.WarningCode = "recipient_forbidden"
			target.WarningMessage = "You do not have access to the mentioned squad lead."
			return target
		}
		target.ResolvedAgentID = squad.LeaderID
		return target
	default:
		target.Status = chatRoutingStatusBlocked
		target.WarningCode = "unsupported_recipient"
		target.WarningMessage = "Mentioned recipient type is not supported in chat."
		return target
	}
}

func chatNeedsTargetResponse(state db.ChatSessionDirectedState) *ChatNeedsTargetResponse {
	resp := &ChatNeedsTargetResponse{
		Error:      "needs_target",
		Code:       "needs_target",
		Candidates: []ChatNeedsTargetCandidateResponse{},
	}
	var candidates []chatDirectedStateCandidate
	if len(state.CandidateRecipients) > 0 {
		_ = json.Unmarshal(state.CandidateRecipients, &candidates)
	}
	for _, candidate := range candidates {
		resp.Candidates = append(resp.Candidates, ChatNeedsTargetCandidateResponse{
			RecipientType:   candidate.RecipientType,
			RecipientID:     candidate.RecipientID,
			ResolvedAgentID: candidate.ResolvedAgentID,
		})
	}
	return resp
}

func (h *Handler) createChatRecipientEdge(ctx context.Context, session db.ChatSession, messageID pgtype.UUID, target chatRoutingTarget, taskID pgtype.UUID) (db.ChatMessageRecipient, error) {
	status := target.Status
	if status == "" {
		status = chatRoutingStatusRouted
	}
	source := target.Source
	if source == "" {
		source = chatRoutingSourceExplicitMention
	}
	return h.Queries.CreateChatMessageRecipient(ctx, db.CreateChatMessageRecipientParams{
		WorkspaceID:     session.WorkspaceID,
		ChatSessionID:   session.ID,
		MessageID:       messageID,
		RecipientType:   target.RecipientType,
		RecipientID:     target.RecipientID,
		ResolvedAgentID: target.ResolvedAgentID,
		Source:          source,
		Status:          status,
		RecipientTaskID: taskID,
		WarningCode:     target.WarningCode,
		WarningMessage:  target.WarningMessage,
	})
}

func (h *Handler) updateChatDirectedStateForTargets(ctx context.Context, session db.ChatSession, messageID pgtype.UUID, targets []chatRoutingTarget) {
	routable := make([]chatRoutingTarget, 0, len(targets))
	for _, target := range targets {
		if target.ResolvedAgentID.Valid && target.Status != chatRoutingStatusBlocked {
			routable = append(routable, target)
		}
	}
	if len(routable) == 0 {
		return
	}
	if len(routable) == 1 {
		target := routable[0]
		activeType := target.RecipientType
		activeID := target.RecipientID
		if target.ResolvedAgentID.Valid {
			activeType = "agent"
			activeID = target.ResolvedAgentID
		}
		_, _ = h.Queries.UpsertChatSessionDirectedState(ctx, db.UpsertChatSessionDirectedStateParams{
			ChatSessionID:       session.ID,
			WorkspaceID:         session.WorkspaceID,
			State:               "active",
			ActiveRecipientType: pgtype.Text{String: activeType, Valid: true},
			ActiveRecipientID:   activeID,
			ActiveMessageID:     messageID,
			CandidateRecipients: []byte("[]"),
		})
		return
	}
	candidates := make([]chatDirectedStateCandidate, 0, len(routable))
	for _, target := range routable {
		candidate := chatDirectedStateCandidate{
			RecipientType: target.RecipientType,
			RecipientID:   uuidToString(target.RecipientID),
		}
		if target.ResolvedAgentID.Valid {
			resolved := uuidToString(target.ResolvedAgentID)
			candidate.ResolvedAgentID = &resolved
		}
		candidates = append(candidates, candidate)
	}
	rawCandidates, _ := json.Marshal(candidates)
	_, _ = h.Queries.UpsertChatSessionDirectedState(ctx, db.UpsertChatSessionDirectedStateParams{
		ChatSessionID:       session.ID,
		WorkspaceID:         session.WorkspaceID,
		State:               "ambiguous",
		ActiveRecipientType: pgtype.Text{},
		ActiveRecipientID:   pgtype.UUID{},
		ActiveMessageID:     messageID,
		CandidateRecipients: rawCandidates,
	})
}
