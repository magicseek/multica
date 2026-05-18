import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import { chatKeys } from "./queries";
import { issueKeys } from "../issues/queries";
import { labelKeys } from "../labels/queries";
import { createLogger } from "../logger";
import type { ChatSession, UpdateChatIssueProposalItemRequest } from "../types";

const logger = createLogger("chat.mut");

export function useCreateChatSession() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();

  return useMutation({
    mutationFn: (data: {
      agent_id: string;
      title?: string;
      default_repository_id?: string | null;
      project_id?: string | null;
    }) => {
      logger.info("createChatSession.start", { agent_id: data.agent_id, titleLength: data.title?.length ?? 0 });
      return api.createChatSession(data);
    },
    onSuccess: (session) => {
      logger.info("createChatSession.success", { sessionId: session.id, agentId: session.agent_id });
    },
    onError: (err) => {
      logger.error("createChatSession.error", err);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: chatKeys.sessions(wsId) });
      qc.invalidateQueries({ queryKey: chatKeys.sidebar(wsId) });
    },
  });
}

/**
 * Clears the session's unread state server-side. Optimistically flips
 * has_unread to false in the cached list so the FAB badge drops
 * immediately. The server broadcasts chat:session_read so other devices
 * also sync.
 */
export function useMarkChatSessionRead() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();

  return useMutation({
    mutationFn: (sessionId: string) => {
      logger.info("markChatSessionRead.start", { sessionId });
      return api.markChatSessionRead(sessionId);
    },
    onMutate: async (sessionId) => {
      await qc.cancelQueries({ queryKey: chatKeys.sessions(wsId) });

      const prevSessions = qc.getQueryData<ChatSession[]>(chatKeys.sessions(wsId));

      const clear = (old?: ChatSession[]) =>
        old?.map((s) => (s.id === sessionId ? { ...s, has_unread: false } : s));
      qc.setQueryData<ChatSession[]>(chatKeys.sessions(wsId), clear);

      return { prevSessions };
    },
    onError: (err, sessionId, ctx) => {
      logger.error("markChatSessionRead.error.rollback", { sessionId, err });
      if (ctx?.prevSessions) qc.setQueryData(chatKeys.sessions(wsId), ctx.prevSessions);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: chatKeys.sessions(wsId) });
      qc.invalidateQueries({ queryKey: chatKeys.sidebar(wsId) });
    },
  });
}

/**
 * Renames a chat session. Optimistically swaps the title in the cached
 * list so the dropdown reflects the new label immediately; rolls back on
 * error. The matching `chat:session_updated` WS event keeps other
 * tabs/devices in sync — see use-realtime-sync.ts.
 */
export function useUpdateChatSession() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();

  return useMutation({
    mutationFn: (data: {
      sessionId: string;
      title?: string;
      default_repository_id?: string | null;
    }) => {
      logger.info("updateChatSession.start", {
        sessionId: data.sessionId,
        titleLength: data.title?.length ?? 0,
        updatesRepository: "default_repository_id" in data,
      });
      const { sessionId, ...patch } = data;
      return api.updateChatSession(sessionId, patch);
    },
    onMutate: async ({ sessionId, title, default_repository_id }) => {
      await qc.cancelQueries({ queryKey: chatKeys.sessions(wsId) });

      const prevSessions = qc.getQueryData<ChatSession[]>(chatKeys.sessions(wsId));

      const patch = (old?: ChatSession[]) =>
        old?.map((s) =>
          s.id === sessionId
            ? {
                ...s,
                ...(title !== undefined ? { title } : {}),
                ...(default_repository_id !== undefined ? { default_repository_id } : {}),
              }
            : s,
        );
      qc.setQueryData<ChatSession[]>(chatKeys.sessions(wsId), patch);

      return { prevSessions };
    },
    onError: (err, vars, ctx) => {
      logger.error("updateChatSession.error.rollback", { sessionId: vars.sessionId, err });
      if (ctx?.prevSessions) qc.setQueryData(chatKeys.sessions(wsId), ctx.prevSessions);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: chatKeys.sessions(wsId) });
      qc.invalidateQueries({ queryKey: chatKeys.sidebar(wsId) });
    },
  });
}

/**
 * Archives a chat session. Optimistically removes the row from the
 * sessions list so the dropdown updates instantly; rolls back on error.
 * The matching `chat:session_archived` WS event keeps other tabs/devices
 * in sync — see use-realtime-sync.ts.
 */
export function useDeleteChatSession() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();

  return useMutation({
    mutationFn: (sessionId: string) => {
      logger.info("deleteChatSession.start", { sessionId });
      return api.deleteChatSession(sessionId);
    },
    onMutate: async (sessionId) => {
      await qc.cancelQueries({ queryKey: chatKeys.sessions(wsId) });

      const prevSessions = qc.getQueryData<ChatSession[]>(chatKeys.sessions(wsId));

      const drop = (old?: ChatSession[]) => old?.filter((s) => s.id !== sessionId);
      qc.setQueryData<ChatSession[]>(chatKeys.sessions(wsId), drop);

      logger.debug("deleteChatSession.optimistic", { sessionId });
      return { prevSessions };
    },
    onError: (err, sessionId, ctx) => {
      logger.error("deleteChatSession.error.rollback", { sessionId, err });
      if (ctx?.prevSessions) qc.setQueryData(chatKeys.sessions(wsId), ctx.prevSessions);
    },
    onSettled: (_data, _err, sessionId) => {
      logger.debug("deleteChatSession.settled", { sessionId });
      qc.invalidateQueries({ queryKey: chatKeys.sessions(wsId) });
      qc.invalidateQueries({ queryKey: chatKeys.sidebar(wsId) });
    },
  });
}

export function useUpdateChatIssueProposalItem(sessionId: string) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: (data: {
      proposalId: string;
      itemId: string;
      patch: UpdateChatIssueProposalItemRequest;
    }) => {
      logger.info("updateChatIssueProposalItem.start", {
        proposalId: data.proposalId,
        itemId: data.itemId,
      });
      return api.updateChatIssueProposalItem(data.proposalId, data.itemId, data.patch);
    },
    onError: (err, vars) => {
      logger.error("updateChatIssueProposalItem.error", {
        proposalId: vars.proposalId,
        itemId: vars.itemId,
        err,
      });
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: chatKeys.issueProposals(sessionId) });
    },
  });
}

export function useApproveChatIssueProposal(sessionId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();

  return useMutation({
    mutationFn: (data: { proposalId: string; itemIds: string[] }) => {
      logger.info("approveChatIssueProposal.start", {
        proposalId: data.proposalId,
        count: data.itemIds.length,
      });
      return api.approveChatIssueProposal(data.proposalId, data.itemIds);
    },
    onError: (err, vars) => {
      logger.error("approveChatIssueProposal.error", {
        proposalId: vars.proposalId,
        err,
      });
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: chatKeys.issueProposals(sessionId) });
      qc.invalidateQueries({ queryKey: chatKeys.issues(sessionId) });
      qc.invalidateQueries({ queryKey: issueKeys.all(wsId) });
      qc.invalidateQueries({ queryKey: labelKeys.list(wsId) });
    },
  });
}

export function useRestoreChatIssueProposalItem(sessionId: string) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: (data: { proposalId: string; itemId: string }) => {
      logger.info("restoreChatIssueProposalItem.start", data);
      return api.restoreChatIssueProposalItem(data.proposalId, data.itemId);
    },
    onError: (err, vars) => {
      logger.error("restoreChatIssueProposalItem.error", {
        proposalId: vars.proposalId,
        itemId: vars.itemId,
        err,
      });
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: chatKeys.issueProposals(sessionId) });
    },
  });
}

export function useDismissChatIssueProposal(sessionId: string) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: (proposalId: string) => {
      logger.info("dismissChatIssueProposal.start", { proposalId });
      return api.dismissChatIssueProposal(proposalId);
    },
    onError: (err, proposalId) => {
      logger.error("dismissChatIssueProposal.error", { proposalId, err });
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: chatKeys.issueProposals(sessionId) });
    },
  });
}
