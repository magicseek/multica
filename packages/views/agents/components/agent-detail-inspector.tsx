"use client";

import {
  useEffect,
  useId,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { Camera, Info, Loader2, Pencil } from "lucide-react";
import { toast } from "sonner";
import type {
  Agent,
  AgentRuntime,
  ExecutionProtocolSlug,
  MemberWithUser,
} from "@multica/core/types";
import {
  AGENT_DESCRIPTION_MAX_LENGTH,
  type AgentPresenceDetail,
} from "@multica/core/agents";
import { api } from "@multica/core/api";
import { useFileUpload } from "@multica/core/hooks/use-file-upload";
import { isImeComposing, timeAgo } from "@multica/core/utils";
import { Button } from "@multica/ui/components/ui/button";
import { ActorAvatar } from "../../common/actor-avatar";
import { Input } from "@multica/ui/components/ui/input";
import { Switch } from "@multica/ui/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@multica/ui/components/ui/popover";
import { PropRow } from "../../common/prop-row";
import { availabilityConfig } from "../presence";
import { CharCounter } from "./char-counter";
import { useT } from "../../i18n";
import { ConcurrencyPicker } from "./inspector/concurrency-picker";
import { ModelPicker } from "./inspector/model-picker";
import { RuntimePicker } from "./inspector/runtime-picker";
import { SkillAttach } from "./inspector/skill-attach";
import { VisibilityPicker } from "./inspector/visibility-picker";

const STANDARD_EXECUTION_PROTOCOL = "standard-assignment" satisfies ExecutionProtocolSlug;
const TRELLIS_EXECUTION_PROTOCOL = "trellis-task" satisfies ExecutionProtocolSlug;
type ExecutionProtocolSelectValue = "off" | typeof STANDARD_EXECUTION_PROTOCOL | typeof TRELLIS_EXECUTION_PROTOCOL;

function executionProtocolValue(agent: Agent): ExecutionProtocolSelectValue {
  if (agent.execution_protocol_enabled !== true) {
    return "off";
  }
  return agent.execution_protocol_slug === TRELLIS_EXECUTION_PROTOCOL
    ? TRELLIS_EXECUTION_PROTOCOL
    : STANDARD_EXECUTION_PROTOCOL;
}

function isRequestEfficientRecommendedProvider(provider?: string | null): boolean {
  const normalized = provider?.trim().toLowerCase() ?? "";
  return normalized.includes("copilot") || normalized.includes("kiro");
}

interface InspectorProps {
  agent: Agent;
  runtime: AgentRuntime | null;
  owner: MemberWithUser | null;
  presence: AgentPresenceDetail | null | undefined;
  // Below: needed for inline edit. The inspector now owns the editing surface
  // (no Settings tab anymore), so the parent has to pass through everything
  // a write needs.
  runtimes: AgentRuntime[];
  members: MemberWithUser[];
  currentUserId: string | null;
  /**
   * Computed by the parent via `useAgentPermissions(agent).canEdit.allowed`.
   * When false the inspector renders all editable surfaces as static
   * read-only displays — pickers become text/badges, name/description lose
   * their pencil affordance, the avatar is no longer clickable, and the
   * "Attach skill" trigger is hidden. Mirrors the backend gate at
   * `server/internal/handler/agent.go:519-535`.
   */
  canEdit: boolean;
  onUpdate: (id: string, data: Record<string, unknown>) => Promise<void>;
}

/**
 * Left 320px column of the agent detail page. Holds the agent's identity card
 * (avatar / name / description / status), inline-editable properties, and
 * skills.
 *
 * **All editing happens here** — there is no separate Settings tab. The
 * trade-off is that the inspector carries some weight (4 inline pickers plus
 * 3 popovers for name/description/avatar), but it eliminates the "see vs
 * edit" mode split that the previous Settings tab created. Users no longer
 * have to switch tabs and hunt for the field they were already looking at.
 */
export function AgentDetailInspector({
  agent,
  runtime,
  owner,
  presence,
  runtimes,
  members,
  currentUserId,
  canEdit,
  onUpdate,
}: InspectorProps) {
  const { t } = useT("agents");
  const update = (data: Record<string, unknown>) => onUpdate(agent.id, data);
  const isOnline = runtime?.status === "online";
  const requestEfficientHelpText = isRequestEfficientRecommendedProvider(
    runtime?.provider,
  )
    ? t(($) => $.request_efficient.recommended_hint)
    : t(($) => $.request_efficient.hint);

  return (
    <aside className="flex w-full flex-col rounded-lg border bg-background md:h-full md:min-h-0 md:overflow-y-auto">
      {/* Identity */}
      <div className="flex flex-col gap-3 border-b px-5 pb-5 pt-5">
        <AvatarEditor agent={agent} canEdit={canEdit} onUpdate={update} />
        <NameAndDescription
          agent={agent}
          canEdit={canEdit}
          onUpdate={update}
        />
        <PresenceBadge presence={presence} />
      </div>

      {/* Properties — editable when canEdit. When the current user lacks
          permission, each picker self-renders a static read-only display so
          the value is visible but not interactive. */}
      <Section label={t(($) => $.inspector.section_properties)}>
        <PropRow label={t(($) => $.inspector.prop_runtime)} interactive={false}>
          <RuntimePicker
            value={agent.runtime_id}
            runtimes={runtimes}
            members={members}
            currentUserId={currentUserId}
            canEdit={canEdit}
            onChange={(id) => update({ runtime_id: id })}
          />
        </PropRow>
        <PropRow label={t(($) => $.inspector.prop_model)} interactive={false}>
          <ModelPicker
            runtimeId={agent.runtime_id}
            runtimeOnline={!!isOnline}
            value={agent.model ?? ""}
            canEdit={canEdit}
            onChange={(m) => update({ model: m })}
          />
        </PropRow>
        <PropRow label={t(($) => $.inspector.prop_visibility)} interactive={false}>
          <VisibilityPicker
            value={agent.visibility}
            canEdit={canEdit}
            onChange={(v) => update({ visibility: v })}
          />
        </PropRow>
        <PropRow label={t(($) => $.inspector.prop_concurrency)} interactive={false}>
          <ConcurrencyPicker
            value={agent.max_concurrent_tasks}
            canEdit={canEdit}
            onChange={(n) => update({ max_concurrent_tasks: n })}
          />
        </PropRow>
        <PropRow
          label={t(($) => $.inspector.prop_execution_protocol)}
          interactive={false}
        >
          <Select
            value={executionProtocolValue(agent)}
            onValueChange={(value) => {
              if (
                value !== "off" &&
                value !== STANDARD_EXECUTION_PROTOCOL &&
                value !== TRELLIS_EXECUTION_PROTOCOL
              ) {
                return;
              }
              const slug: ExecutionProtocolSlug = value === "off" ? "" : value;
              update({
                execution_protocol_enabled: slug !== "",
                execution_protocol_slug: slug,
              });
            }}
            aria-label={t(($) => $.inspector.execution_protocol_aria)}
          >
            <SelectTrigger
              size="sm"
              className="w-32"
              disabled={!canEdit}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="off">
                {t(($) => $.execution_protocol.option_off)}
              </SelectItem>
              <SelectItem value={STANDARD_EXECUTION_PROTOCOL}>
                {t(($) => $.execution_protocol.option_standard)}
              </SelectItem>
              <SelectItem value={TRELLIS_EXECUTION_PROTOCOL}>
                {t(($) => $.execution_protocol.option_trellis)}
              </SelectItem>
            </SelectContent>
          </Select>
        </PropRow>
        <div className="-mx-2 col-span-2 grid min-h-8 grid-cols-subgrid items-center rounded-md px-2">
          <span className="text-xs text-muted-foreground">
            {t(($) => $.inspector.prop_request_efficient)}
          </span>
          <RequestEfficientControl
            text={requestEfficientHelpText}
            checked={agent.request_efficient_enabled === true}
            disabled={!canEdit}
            switchLabel={t(($) => $.inspector.request_efficient_aria)}
            recommended={isRequestEfficientRecommendedProvider(runtime?.provider)}
            recommendedLabel={t(($) => $.request_efficient.recommended_badge)}
            onCheckedChange={(checked) => update({ request_efficient_enabled: checked })}
          />
        </div>
      </Section>

      {/* Details — read-only (no hover, no chip styling — these aren't clickable) */}
      <Section label={t(($) => $.inspector.section_details)}>
        {owner && (
          <PropRow label={t(($) => $.inspector.prop_owner)} interactive={false}>
            <span className="flex min-w-0 items-center gap-1.5">
              <ActorAvatar
                actorType="member"
                actorId={owner.user_id}
                size={14}
              />
              <span className="truncate">{owner.name}</span>
            </span>
          </PropRow>
        )}
        <PropRow label={t(($) => $.inspector.prop_created)} interactive={false}>
          <span className="text-muted-foreground">
            {timeAgo(agent.created_at)}
          </span>
        </PropRow>
        <PropRow label={t(($) => $.inspector.prop_updated)} interactive={false}>
          <span className="text-muted-foreground">
            {timeAgo(agent.updated_at)}
          </span>
        </PropRow>
      </Section>

      {/* Skills */}
      <div className="flex flex-col border-b px-5 py-4">
        <div className="mb-2 flex items-center gap-2">
          <span className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
            {t(($) => $.inspector.section_skills)}
          </span>
          <span className="font-mono text-[10px] tabular-nums text-muted-foreground/70">
            {agent.skills.length}
          </span>
        </div>
        <div className="flex flex-wrap gap-1">
          {agent.skills.map((s) => (
            <span
              key={s.id}
              className="rounded-md bg-muted px-1.5 py-0.5 font-mono text-[10px] font-medium text-muted-foreground"
            >
              {s.name}
            </span>
          ))}
          <SkillAttach agent={agent} canEdit={canEdit} />
        </div>
      </div>
    </aside>
  );
}

function RequestEfficientControl({
  text,
  checked,
  disabled,
  switchLabel,
  recommended,
  recommendedLabel,
  onCheckedChange,
}: {
  text: string;
  checked: boolean;
  disabled: boolean;
  switchLabel: string;
  recommended: boolean;
  recommendedLabel: string;
  onCheckedChange: (checked: boolean) => void;
}) {
  const [open, setOpen] = useState(false);
  const helpId = useId();
  const closeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clearCloseTimer = () => {
    if (closeTimerRef.current) {
      clearTimeout(closeTimerRef.current);
      closeTimerRef.current = null;
    }
  };

  const openHelp = () => {
    clearCloseTimer();
    setOpen(true);
  };

  const scheduleClose = () => {
    clearCloseTimer();
    closeTimerRef.current = setTimeout(() => setOpen(false), 150);
  };

  useEffect(() => {
    return () => {
      if (closeTimerRef.current) {
        clearTimeout(closeTimerRef.current);
      }
    };
  }, []);

  return (
    <div
      className="flex min-w-0 flex-col items-start gap-1.5 text-xs"
      onMouseEnter={openHelp}
      onMouseLeave={scheduleClose}
    >
      <div className="flex min-w-0 items-center gap-2">
        <button
          type="button"
          title={text}
          aria-label={text}
          aria-describedby={open ? helpId : undefined}
          aria-expanded={open}
          className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          onClick={(event) => {
            event.preventDefault();
            clearCloseTimer();
            setOpen(true);
          }}
          onFocus={openHelp}
          onBlur={scheduleClose}
        >
          <Info className="h-3.5 w-3.5" />
        </button>
        <Switch
          checked={checked}
          onCheckedChange={onCheckedChange}
          disabled={disabled}
          aria-label={switchLabel}
        />
        {recommended && (
          <span className="truncate text-[11px] text-muted-foreground">
            {recommendedLabel}
          </span>
        )}
      </div>
      {open && (
        <div
          id={helpId}
          role="tooltip"
          className="max-w-64 rounded-lg border bg-popover p-2.5 text-left text-xs leading-relaxed text-popover-foreground shadow-sm"
        >
          {text}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Layout helpers
// ---------------------------------------------------------------------------

function Section({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="border-b px-5 py-4">
      <div className="mb-1 -mx-2 px-2 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
        {label}
      </div>
      <div className="grid grid-cols-[auto_1fr] gap-x-2 gap-y-0.5">
        {children}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Identity — avatar / name / description editors
// ---------------------------------------------------------------------------

function AvatarEditor({
  agent,
  canEdit,
  onUpdate,
}: {
  agent: Agent;
  canEdit: boolean;
  onUpdate: (data: Record<string, unknown>) => Promise<void>;
}) {
  const { t } = useT("agents");
  const fileInputRef = useRef<HTMLInputElement>(null);
  const { upload, uploading } = useFileUpload(api);

  if (!canEdit) {
    return (
      <div className="h-14 w-14 shrink-0 overflow-hidden rounded-lg bg-muted">
        <ActorAvatar
          actorType="agent"
          actorId={agent.id}
          size={56}
          className="rounded-none"
        />
      </div>
    );
  }

  const handleFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    e.target.value = "";
    try {
      const result = await upload(file);
      if (!result) return;
      await onUpdate({ avatar_url: result.link });
      toast.success(t(($) => $.inspector.avatar_updated_toast));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t(($) => $.inspector.avatar_upload_failed_toast));
    }
  };

  return (
    <>
      <button
        type="button"
        // rounded-lg matches the standard agent avatar treatment used in
        // list rows. Avoid rounded-full — circles are reserved for humans.
        className="group relative h-14 w-14 shrink-0 overflow-hidden rounded-lg bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        onClick={() => fileInputRef.current?.click()}
        disabled={uploading}
        aria-label={t(($) => $.inspector.change_avatar_aria)}
      >
        <ActorAvatar
          actorType="agent"
          actorId={agent.id}
          size={56}
          className="rounded-none"
        />
        <div className="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 transition-opacity group-hover:opacity-100">
          {uploading ? (
            <Loader2 className="h-4 w-4 animate-spin text-white" />
          ) : (
            <Camera className="h-4 w-4 text-white" />
          )}
        </div>
      </button>
      <input
        ref={fileInputRef}
        type="file"
        accept="image/*"
        className="hidden"
        onChange={handleFile}
      />
    </>
  );
}

function NameAndDescription({
  agent,
  canEdit,
  onUpdate,
}: {
  agent: Agent;
  canEdit: boolean;
  onUpdate: (data: Record<string, unknown>) => Promise<void>;
}) {
  const { t } = useT("agents");
  if (!canEdit) {
    return (
      <div className="flex flex-col gap-1">
        <span className="text-base font-semibold leading-tight">
          {agent.name}
        </span>
        {agent.description ? (
          <span className="text-xs leading-relaxed text-muted-foreground">
            {agent.description}
          </span>
        ) : (
          <span className="text-xs italic leading-relaxed text-muted-foreground/50">
            {t(($) => $.inspector.no_description_placeholder)}
          </span>
        )}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-1">
      <InlineEditPopover
        value={agent.name}
        onSave={(v) => onUpdate({ name: v.trim() })}
        kind="input"
        title={t(($) => $.inspector.rename_title)}
        placeholder={t(($) => $.inspector.rename_placeholder)}
        validate={(v) => (v.trim().length > 0 ? null : t(($) => $.inspector.rename_required))}
      >
        {(triggerProps) => (
          <button
            type="button"
            {...triggerProps}
            className="group -mx-1 inline-flex items-center gap-1.5 self-start rounded px-1 text-left text-base font-semibold leading-tight transition-colors hover:bg-accent/50"
          >
            <span>{agent.name}</span>
            <Pencil className="h-3 w-3 shrink-0 text-muted-foreground/0 transition-colors group-hover:text-muted-foreground" />
          </button>
        )}
      </InlineEditPopover>

      <DescriptionEditor
        value={agent.description ?? ""}
        onSave={(v) => onUpdate({ description: v })}
      />
    </div>
  );
}

// Description editor — modal because the description benefits from a roomy
// composition surface (the inline popover was 288 px wide × 3 rows, too
// cramped to read or edit anything substantial). Name stays in the inline
// popover above: a single line is the right shape for it.
//
// The editor body is split into a child component that mounts only while
// the dialog is open. That way the draft state is initialised from `value`
// at mount time and never reset by an external update mid-edit — closing
// the dialog unmounts the body, reopening starts fresh with the latest
// value. This is the React-recommended replacement for the
// `useEffect(reset, [value])` anti-pattern (see "You Might Not Need an
// Effect" — Resetting state with a key / mount).
function DescriptionEditor({
  value,
  onSave,
}: {
  value: string;
  onSave: (next: string) => Promise<void>;
}) {
  const { t } = useT("agents");
  const [open, setOpen] = useState(false);

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="group -mx-1 inline-flex items-start gap-1.5 self-start rounded px-1 text-left text-xs leading-relaxed transition-colors hover:bg-accent/50"
      >
        {value ? (
          <span className="text-muted-foreground">{value}</span>
        ) : (
          <span className="italic text-muted-foreground/50">{t(($) => $.inspector.no_description_placeholder)}</span>
        )}
        <Pencil className="mt-0.5 h-3 w-3 shrink-0 text-muted-foreground/0 transition-colors group-hover:text-muted-foreground" />
      </button>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-lg">
          {open && (
            <DescriptionEditorBody
              initialValue={value}
              onSave={onSave}
              onClose={() => setOpen(false)}
            />
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}

function DescriptionEditorBody({
  initialValue,
  onSave,
  onClose,
}: {
  initialValue: string;
  onSave: (next: string) => Promise<void>;
  onClose: () => void;
}) {
  const { t } = useT("agents");
  const [draft, setDraft] = useState(initialValue);
  const [saving, setSaving] = useState(false);

  const length = [...draft].length;
  const overLimit = length > AGENT_DESCRIPTION_MAX_LENGTH;
  const dirty = draft !== initialValue;

  const commit = async () => {
    if (overLimit || !dirty) return;
    setSaving(true);
    try {
      await onSave(draft);
      onClose();
    } catch {
      // toast handled by parent's onUpdate
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t(($) => $.inspector.edit_description_title)}</DialogTitle>
      </DialogHeader>
      <div className="flex flex-col gap-2">
        <textarea
          autoFocus
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          placeholder={t(($) => $.inspector.description_placeholder)}
          rows={6}
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              onClose();
              return;
            }
            if (isImeComposing(e)) return;
            if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
              e.preventDefault();
              void commit();
            }
          }}
          className="w-full resize-none rounded-md border bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-input"
        />
        <CharCounter length={length} max={AGENT_DESCRIPTION_MAX_LENGTH} />
      </div>
      <DialogFooter>
        <Button
          variant="ghost"
          size="sm"
          onClick={onClose}
          disabled={saving}
        >
          {t(($) => $.inspector.cancel)}
        </Button>
        <Button
          size="sm"
          onClick={() => void commit()}
          disabled={saving || overLimit || !dirty}
        >
          {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : t(($) => $.inspector.save)}
        </Button>
      </DialogFooter>
    </>
  );
}


// Generic single-field popover editor used for name / description. Keeps the
// trigger styling fully in the caller's hands by using a render prop.
function InlineEditPopover({
  value,
  onSave,
  kind,
  title,
  placeholder,
  validate,
  children,
}: {
  value: string;
  onSave: (next: string) => Promise<void>;
  kind: "input" | "textarea";
  title: string;
  placeholder?: string;
  validate?: (v: string) => string | null;
  children: (triggerProps: {
    onClick: (e: React.MouseEvent) => void;
  }) => ReactNode;
}) {
  const { t } = useT("agents");
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Reset draft when popover opens or upstream value changes between sessions.
  useEffect(() => {
    if (open) {
      setDraft(value);
      setError(null);
    }
  }, [open, value]);

  const commit = async () => {
    const err = validate?.(draft) ?? null;
    if (err) {
      setError(err);
      return;
    }
    if (draft === value) {
      setOpen(false);
      return;
    }
    setSaving(true);
    try {
      await onSave(draft);
      setOpen(false);
    } catch {
      // toast handled by parent's onUpdate
    } finally {
      setSaving(false);
    }
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={children({ onClick: () => setOpen(true) }) as React.ReactElement}
      />
      <PopoverContent align="start" className="w-72 p-3">
        <div className="space-y-2">
          <p className="text-xs font-medium">{title}</p>
          {kind === "input" ? (
            <Input
              autoFocus
              value={draft}
              onChange={(e) => {
                setDraft(e.target.value);
                if (error) setError(null);
              }}
              placeholder={placeholder}
              onKeyDown={(e) => {
                if (e.key === "Escape") {
                  setOpen(false);
                  return;
                }
                if (isImeComposing(e)) return;
                if (e.key === "Enter") {
                  e.preventDefault();
                  void commit();
                }
              }}
              className="h-8"
            />
          ) : (
            <textarea
              autoFocus
              value={draft}
              onChange={(e) => {
                setDraft(e.target.value);
                if (error) setError(null);
              }}
              placeholder={placeholder}
              onKeyDown={(e) => {
                if (e.key === "Escape") {
                  setOpen(false);
                  return;
                }
                if (isImeComposing(e)) return;
                if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                  e.preventDefault();
                  void commit();
                }
              }}
              rows={3}
              className="w-full resize-none rounded-md border bg-transparent px-2 py-1.5 text-xs outline-none focus-visible:border-input"
            />
          )}
          {error && <p className="text-xs text-destructive">{error}</p>}
          <div className="flex items-center justify-end gap-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setOpen(false)}
              disabled={saving}
            >
              {t(($) => $.inspector.cancel)}
            </Button>
            <Button
              size="sm"
              onClick={() => void commit()}
              disabled={saving || draft === value}
            >
              {saving ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                t(($) => $.inspector.save)
              )}
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}

// ---------------------------------------------------------------------------
// Presence badge — unchanged from the previous version
// ---------------------------------------------------------------------------

function PresenceBadge({
  presence,
}: {
  presence: AgentPresenceDetail | null | undefined;
}) {
  const { t } = useT("agents");
  if (!presence) {
    return (
      <span className="inline-flex h-5 w-20 animate-pulse rounded-md bg-muted" />
    );
  }
  const av = availabilityConfig[presence.availability];
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <span
        className={`inline-flex items-center gap-1.5 rounded-md border px-1.5 py-0.5 text-xs ${av.textClass}`}
      >
        <span className={`h-1.5 w-1.5 rounded-full ${av.dotClass}`} />
        {t(($) => $.availability[presence.availability])}
      </span>
    </div>
  );
}
