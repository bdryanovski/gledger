# DoubleBook — Development Work Process

## Overview

This document describes the workflow for building DoubleBook. Tasks are broken into small,
focused units that each represent a single coherent piece of work. Every task has clear
acceptance criteria so both the developer (AI) and the reviewer (human) can confidently
verify it is complete before moving on.

---

## Folder Structure

```
tasks/
├── PROCESS.md          <- this file
├── T1.1-rename.md
├── T1.2-bug-fixes.md
├── ...
└── done/               <- completed and validated tasks are moved here
    ├── T1.1-rename.md
    └── ...
```

---

## Task Lifecycle

```
┌─────────────┐     implement     ┌──────────────┐     human validates     ┌──────────────┐
│  tasks/     │  ─────────────►  │  task done,  │  ───────────────────►  │  tasks/done/ │
│  T?.?.md    │                  │  moved to    │                         │  T?.?.md     │
│  (pending)  │                  │  done/       │                         │  (archived)  │
└─────────────┘                  └──────────────┘                         └──────────────┘
```

### Step-by-step

1. **Pick the next task** — Tasks are worked in order: T1.1 → T1.2 → T1.3 → ...
   Dependencies between tasks are noted in each task file.

2. **Implement** — Follow the task spec exactly. Each task file lists:
   - Goal (one sentence)
   - Context (why this matters, what it depends on)
   - Detailed steps to implement
   - Files to create or modify
   - Acceptance criteria (checkboxes)

3. **Self-verify** — Run any listed verification commands (build, test) and confirm
   all acceptance criteria are met before declaring done.

4. **Move to done** — Move the completed task file from `tasks/` to `tasks/done/`.

5. **Stop and report** — Write a short summary of what was done and wait for the human
   to validate. Do not start the next task until the human gives the go-ahead.

6. **Human validates** — Reviews the changes, tests manually, and says "continue" (or
   flags issues to fix first).

---

## Task Naming Convention

```
T{phase}.{number}-{short-slug}.md
```

Examples:
- `T1.1-rename.md` — Phase 1, task 1: rename project
- `T2.3-csv-importer.md` — Phase 2, task 3: CSV importer
- `T3.4-fql-compiler.md` — Phase 3, task 4: FQL compiler

---

## Phase Summary

| Phase | Theme | Tasks |
|-------|-------|-------|
| 1 | Foundation & Core | T1.1 – T1.12 |
| 2 | Import & Interactive Insert | T2.1 – T2.4 |
| 3 | FQL + SQLite | T3.1 – T3.7 |
| 4 | Multi-Currency, Web & API | T4.1 – T4.7 |
| 5 | Plugins | T5.1 – T5.4 |

---

## General Implementation Rules

- **Build must pass** after every task. Run `go build ./...` before declaring done.
- **Follow existing code style** — tabs for indentation, same package naming conventions.
- **Do not mix concerns** — each task touches only its stated scope. If something else
  needs fixing along the way, note it but do not fix it unless it blocks the current task.
- **Prefer editing existing files** over creating new ones where possible.
- **No placeholder code** — every function must do real work, no `// TODO` stubs unless
  the task spec explicitly says "stub is acceptable here".
- **Backward compatibility** — the journal file format must remain hledger-compatible
  throughout all changes.
- **Commit hygiene** — each task represents one logical commit (the human decides when
  to actually commit).
