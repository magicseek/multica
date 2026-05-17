# Code Reuse Thinking Guide

> **Purpose**: Stop and think before creating new code - does it already exist?

---

## The Problem

**Duplicated code is the #1 source of inconsistency bugs.**

When you copy-paste or rewrite existing logic:
- Bug fixes don't propagate
- Behavior diverges over time
- Codebase becomes harder to understand

---

## Before Writing New Code

### Step 1: Search First

```bash
# Search for similar function names
grep -r "functionName" .

# Search for similar logic
grep -r "keyword" .
```

### Step 2: Ask These Questions

| Question | If Yes... |
|----------|-----------|
| Does a similar function exist? | Use or extend it |
| Is this pattern used elsewhere? | Follow the existing pattern |
| Could this be a shared utility? | Create it in the right place |
| Am I copying code from another file? | **STOP** - extract to shared |

### Step 3: When Referencing Another Product

When a task says "follow/reference app X", make a behavior inventory before
implementation. Do not stop at visual style or component names.

Required inventory:
- [ ] User-visible surfaces and tabs
- [ ] Data model fields that power each surface
- [ ] Preview/visualization modes, including graph or diagram views
- [ ] Whether preview content is rendered output or raw source text
- [ ] Toolbar affordances such as expand/collapse controls and tab icons
- [ ] Viewport transition behavior after resize, expand, collapse, or split-pane
      changes, including zoom, scroll position, and centering
- [ ] Independent versus shared scroll containers for adjacent preview panes,
      especially when one pane can contain long rendered content
- [ ] List/step ordering cues such as numbering, grouping, and visual density
- [ ] Import/export or schema round-trip behavior
- [ ] Validation and error states
- [ ] Tests or manual QA evidence for each referenced behavior

Every in-scope behavior must become an explicit acceptance item in the local
spec before coding. If a referenced behavior is intentionally out of scope,
record it as rejected with the reason. This prevents "concept copied, behavior
missed" bugs such as implementing workflow Source/Steps/Markdown preview while
omitting the referenced graph preview, or showing "rendered" Markdown as raw
source text.

---

## Common Duplication Patterns

### Pattern 1: Copy-Paste Functions

**Bad**: Copying a validation function to another file

**Good**: Extract to shared utilities, import where needed

### Pattern 2: Similar Components

**Bad**: Creating a new component that's 80% similar to existing

**Good**: Extend existing component with props/variants

### Pattern 3: Repeated Constants

**Bad**: Defining the same constant in multiple files

**Good**: Single source of truth, import everywhere

---

## When to Abstract

**Abstract when**:
- Same code appears 3+ times
- Logic is complex enough to have bugs
- Multiple people might need this

**Don't abstract when**:
- Only used once
- Trivial one-liner
- Abstraction would be more complex than duplication

---

## After Batch Modifications

When you've made similar changes to multiple files:

1. **Review**: Did you catch all instances?
2. **Search**: Run grep to find any missed
3. **Consider**: Should this be abstracted?

---

## Gotcha: Asymmetric Mechanisms Producing Same Output

**Problem**: When two different mechanisms must produce the same file set (e.g., recursive directory copy for init vs. manual `files.set()` for update), structural changes (renaming, moving, adding subdirectories) only propagate through the automatic mechanism. The manual one silently drifts.

**Symptom**: Init works perfectly, but update creates files at wrong paths or misses files entirely.

**Prevention checklist**:
- [ ] When migrating directory structures, search for ALL code paths that reference the old structure
- [ ] If one path is auto-derived (glob/copy) and another is manually listed, the manual one needs updating
- [ ] Add a regression test that compares outputs from both mechanisms

---

## Checklist Before Commit

- [ ] Searched for existing similar code
- [ ] No copy-pasted logic that should be shared
- [ ] Constants defined in one place
- [ ] Similar patterns follow same structure
