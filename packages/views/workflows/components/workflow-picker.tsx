"use client";

import { Check, Workflow, X } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { workflowListOptions } from "@multica/core/workflows";
import type { UpdateIssueRequest } from "@multica/core/types";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { useT } from "../../i18n";

export function WorkflowPicker({
  workflowId,
  onUpdate,
  triggerRender,
  align = "start",
  defaultOpen = false,
}: {
  workflowId: string | null;
  onUpdate: (updates: Partial<UpdateIssueRequest>) => void;
  triggerRender?: React.ReactElement;
  align?: "start" | "center" | "end";
  defaultOpen?: boolean;
}) {
  const { t } = useT("workflows");
  const wsId = useWorkspaceId();
  const { data: workflows = [] } = useQuery(
    workflowListOptions(wsId, { applicability: "assignment" }),
  );
  const current = workflows.find((w) => w.id === workflowId);

  return (
    <DropdownMenu defaultOpen={defaultOpen}>
      <DropdownMenuTrigger
        className={
          triggerRender
            ? undefined
            : "flex items-center gap-1.5 cursor-pointer rounded px-1 -mx-1 hover:bg-accent/30 transition-colors overflow-hidden"
        }
        render={triggerRender}
      >
        <Workflow className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        <span className="truncate">
          {current ? current.name : t(($) => $.picker.project_default)}
        </span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align={align} className="w-60">
        <DropdownMenuItem
          onClick={() => onUpdate({ workflow_override_definition_id: null })}
        >
          <Workflow className="h-3.5 w-3.5 text-muted-foreground" />
          {t(($) => $.picker.project_default)}
          {!workflowId && <Check className="ml-auto h-3.5 w-3.5 shrink-0" />}
        </DropdownMenuItem>
        {workflows.length > 0 && <DropdownMenuSeparator />}
        {workflows.map((workflow) => (
          <DropdownMenuItem
            key={workflow.id}
            onClick={() =>
              onUpdate({ workflow_override_definition_id: workflow.id })
            }
          >
            <Workflow className="h-3.5 w-3.5 text-muted-foreground" />
            <span className="truncate">{workflow.name}</span>
            {workflow.id === workflowId && (
              <Check className="ml-auto h-3.5 w-3.5 shrink-0" />
            )}
          </DropdownMenuItem>
        ))}
        {workflowId && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onClick={() =>
                onUpdate({ workflow_override_definition_id: null })
              }
            >
              <X className="h-3.5 w-3.5 text-muted-foreground" />
              {t(($) => $.picker.clear_override)}
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
