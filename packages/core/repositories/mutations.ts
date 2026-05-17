import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import type {
  CreateRepositoryBindingRequest,
  CreateRepositoryOperationRequest,
  CreateRepositoryRequest,
  ListProjectRepositoriesResponse,
  ListRepositoriesResponse,
  Repository,
  SetProjectRepositoriesRequest,
  UpdateRepositoryRequest,
} from "../types";
import { repositoryKeys } from "./queries";

export function useCreateRepository(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateRepositoryRequest) => api.createRepository(data),
    onSuccess: (created) => {
      qc.setQueryData<ListRepositoriesResponse>(
        repositoryKeys.list(wsId),
        (old) =>
          old && !old.repositories.some((repo) => repo.id === created.id)
            ? {
                ...old,
                repositories: [...old.repositories, created],
                total: old.total + 1,
              }
            : old,
      );
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: repositoryKeys.list(wsId) });
    },
  });
}

export function useUpdateRepository(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string } & UpdateRepositoryRequest) =>
      api.updateRepository(id, data),
    onMutate: async ({ id, ...data }) => {
      await qc.cancelQueries({ queryKey: repositoryKeys.list(wsId) });
      const prevList = qc.getQueryData<ListRepositoriesResponse>(
        repositoryKeys.list(wsId),
      );
      const prevDetail = qc.getQueryData<Repository>(
        repositoryKeys.detail(wsId, id),
      );
      qc.setQueryData<ListRepositoriesResponse>(
        repositoryKeys.list(wsId),
        (old) =>
          old
            ? {
                ...old,
                repositories: old.repositories.map((repo) =>
                  repo.id === id ? { ...repo, ...data } : repo,
                ),
              }
            : old,
      );
      qc.setQueryData<Repository>(
        repositoryKeys.detail(wsId, id),
        (old) => (old ? { ...old, ...data } : old),
      );
      return { prevList, prevDetail, id };
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prevList) qc.setQueryData(repositoryKeys.list(wsId), ctx.prevList);
      if (ctx?.prevDetail) {
        qc.setQueryData(repositoryKeys.detail(wsId, ctx.id), ctx.prevDetail);
      }
    },
    onSettled: (_data, _err, vars) => {
      qc.invalidateQueries({ queryKey: repositoryKeys.list(wsId) });
      qc.invalidateQueries({ queryKey: repositoryKeys.detail(wsId, vars.id) });
    },
  });
}

export function useArchiveRepository(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (repositoryId: string) => api.deleteRepository(repositoryId),
    onMutate: async (repositoryId) => {
      await qc.cancelQueries({ queryKey: repositoryKeys.list(wsId) });
      const prevList = qc.getQueryData<ListRepositoriesResponse>(
        repositoryKeys.list(wsId),
      );
      qc.setQueryData<ListRepositoriesResponse>(
        repositoryKeys.list(wsId),
        (old) =>
          old
            ? {
                ...old,
                repositories: old.repositories.filter(
                  (repo) => repo.id !== repositoryId,
                ),
                total: Math.max(0, old.total - 1),
              }
            : old,
      );
      return { prevList };
    },
    onError: (_err, _repositoryId, ctx) => {
      if (ctx?.prevList) qc.setQueryData(repositoryKeys.list(wsId), ctx.prevList);
    },
    onSettled: (_data, _err, repositoryId) => {
      qc.invalidateQueries({ queryKey: repositoryKeys.list(wsId) });
      qc.invalidateQueries({
        queryKey: repositoryKeys.detail(wsId, repositoryId),
      });
    },
  });
}

export function useCreateRepositoryBinding(wsId: string, repositoryId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateRepositoryBindingRequest) =>
      api.createRepositoryBinding(repositoryId, data),
    onSettled: () => {
      qc.invalidateQueries({
        queryKey: repositoryKeys.bindings(wsId, repositoryId),
      });
      qc.invalidateQueries({ queryKey: repositoryKeys.list(wsId) });
    },
  });
}

export function useCreateRepositoryOperation(wsId: string, repositoryId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateRepositoryOperationRequest) =>
      api.createRepositoryOperation(repositoryId, data),
    onSettled: () => {
      qc.invalidateQueries({
        queryKey: repositoryKeys.operations(wsId, repositoryId),
      });
      qc.invalidateQueries({ queryKey: repositoryKeys.list(wsId) });
    },
  });
}

export function useSetProjectRepositories(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: SetProjectRepositoriesRequest) =>
      api.setProjectRepositories(projectId, data),
    onSuccess: (updated) => {
      qc.setQueryData<ListProjectRepositoriesResponse>(
        repositoryKeys.project(wsId, projectId),
        updated,
      );
    },
    onSettled: () => {
      qc.invalidateQueries({
        queryKey: repositoryKeys.project(wsId, projectId),
      });
    },
  });
}
