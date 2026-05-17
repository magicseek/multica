import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const repositoryKeys = {
  all: (wsId: string) => ["repositories", wsId] as const,
  taskOutputsAll: () => ["task-outputs"] as const,
  taskOutputs: (taskId: string) => [...repositoryKeys.taskOutputsAll(), taskId] as const,
  list: (wsId: string) => [...repositoryKeys.all(wsId), "list"] as const,
  detail: (wsId: string, repositoryId: string) =>
    [...repositoryKeys.all(wsId), "detail", repositoryId] as const,
  bindings: (wsId: string, repositoryId: string) =>
    [...repositoryKeys.detail(wsId, repositoryId), "bindings"] as const,
  operations: (wsId: string, repositoryId: string) =>
    [...repositoryKeys.detail(wsId, repositoryId), "operations"] as const,
  project: (wsId: string, projectId: string) =>
    [...repositoryKeys.all(wsId), "project", projectId] as const,
};

export function repositoryListOptions(wsId: string) {
  return queryOptions({
    queryKey: repositoryKeys.list(wsId),
    queryFn: () => api.listRepositories(),
    enabled: !!wsId,
    select: (data) => data.repositories,
  });
}

export function repositoryDetailOptions(wsId: string, repositoryId: string) {
  return queryOptions({
    queryKey: repositoryKeys.detail(wsId, repositoryId),
    queryFn: () => api.getRepository(repositoryId),
    enabled: !!wsId && !!repositoryId,
  });
}

export function repositoryBindingsOptions(wsId: string, repositoryId: string) {
  return queryOptions({
    queryKey: repositoryKeys.bindings(wsId, repositoryId),
    queryFn: () => api.listRepositoryBindings(repositoryId),
    enabled: !!wsId && !!repositoryId,
    select: (data) => data.bindings,
  });
}

export function repositoryOperationsOptions(wsId: string, repositoryId: string) {
  return queryOptions({
    queryKey: repositoryKeys.operations(wsId, repositoryId),
    queryFn: () => api.listRepositoryOperations(repositoryId),
    enabled: !!wsId && !!repositoryId,
    select: (data) => data.operations,
  });
}

export function projectRepositoriesOptions(wsId: string, projectId: string) {
  return queryOptions({
    queryKey: repositoryKeys.project(wsId, projectId),
    queryFn: () => api.listProjectRepositories(projectId),
    enabled: !!wsId && !!projectId,
    select: (data) => data.repositories,
  });
}

export function taskOutputMetadataOptions(taskId: string) {
  return queryOptions({
    queryKey: repositoryKeys.taskOutputs(taskId),
    queryFn: () => api.listTaskOutputMetadata(taskId),
    enabled: !!taskId,
    select: (data) => data.outputs,
  });
}
