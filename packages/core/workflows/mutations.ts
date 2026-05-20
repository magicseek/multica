import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import type {
  CreateWorkflowRequest,
  ForkWorkflowRequest,
  UpdateWorkflowRequest,
  WorkflowSchemaRequest,
} from "../types";
import { workflowKeys } from "./queries";

export function useCreateWorkflow() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: CreateWorkflowRequest) => api.createWorkflow(data),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: workflowKeys.all(wsId) });
    },
  });
}

export function useUpdateWorkflow() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string } & UpdateWorkflowRequest) =>
      api.updateWorkflow(id, data),
    onSettled: (_data, _err, vars) => {
      qc.invalidateQueries({ queryKey: workflowKeys.detail(wsId, vars.id) });
      qc.invalidateQueries({ queryKey: workflowKeys.all(wsId) });
    },
  });
}

export function useUpdateWorkflowDraft() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string } & WorkflowSchemaRequest) =>
      api.updateWorkflowDraft(id, data),
    onSettled: (_data, _err, vars) => {
      qc.invalidateQueries({ queryKey: workflowKeys.detail(wsId, vars.id) });
      qc.invalidateQueries({ queryKey: workflowKeys.all(wsId) });
    },
  });
}

export function usePublishWorkflow() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({
      id,
      schema,
    }: {
      id: string;
      schema?: WorkflowSchemaRequest["schema"];
    }) => api.publishWorkflow(id, schema ? { schema } : undefined),
    onSettled: (_data, _err, vars) => {
      qc.invalidateQueries({ queryKey: workflowKeys.detail(wsId, vars.id) });
      qc.invalidateQueries({ queryKey: workflowKeys.all(wsId) });
    },
  });
}

export function useDeleteWorkflowDraft() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.deleteWorkflowDraft(id),
    onSettled: (_data, _err, id) => {
      qc.invalidateQueries({ queryKey: workflowKeys.detail(wsId, id) });
      qc.invalidateQueries({ queryKey: workflowKeys.all(wsId) });
    },
  });
}

export function useForkWorkflow() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string } & ForkWorkflowRequest) =>
      api.forkWorkflow(id, data),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: workflowKeys.all(wsId) });
    },
  });
}

export function useDeleteWorkflow() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.deleteWorkflow(id),
    onSettled: (_data, _err, id) => {
      qc.removeQueries({ queryKey: workflowKeys.detail(wsId, id) });
      qc.invalidateQueries({ queryKey: workflowKeys.all(wsId) });
    },
  });
}
