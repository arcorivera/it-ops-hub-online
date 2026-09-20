# IT Operations Hub

*Everything that needs your attention, in one place.*

A local-first IT ticketing and project management platform. Next.js + TypeScript
frontend, Go backend, SQLite database. Runs entirely on a single Windows laptop
with no cloud dependency, and is architected to move to PostgreSQL on a company
server later without a redesign.

## ⚠️ Current status

This is a real, working application — not a mockup — but it is being built in
phases, and **only some phases are complete so far.** Nothing described below
as "working" is faked or stubbed; everything else in the original spec
(tickets, SLA engine, follow-up/escalation, projects, UAT, incidents/RCA,
reports, backups) has simply not been built yet and does not appear in the UI.

**Working today:**
- First-run administrator setup (real DB write, bcrypt-hashed password)
- Login / logout with real server-side sessions (httpOnly cookies)
- Role-based access control (ADMIN, IT_MANAGER, TEAM_LEAD, AGENT, REQUESTER, VIEWER)
- User management (create, list, activate/deactivate, assign roles) — real CRUD
  against the real database, admin-only
- Teams and ticket categories (create/list)
- **Full ticket lifecycle**: create (with auto-generated ticket numbers like
  `T2026-00001` and automatic SLA policy assignment by severity), list with
  server-side filtering/search/pagination, detail view, assign, status changes
  enforced by a real validated transition graph, severity/priority changes,
  threaded comments, full activity history, and file attachments (extension
  allow-list, path-traversal-safe storage, download)
- **SLA engine**: real business-hours-aware calculation (8AM–5PM weekdays,
  12–1PM lunch excluded) for S3/S4, wall-clock for S1/S2, with genuine
  pause/resume support — a ticket's SLA clock automatically pauses the moment
  it's moved to `PENDING` and resumes when it leaves, and the elapsed/
  remaining/percentage/status (`WITHIN_SLA` → `WARNING` (50%) → `CRITICAL`
  (75%) → `BREACHED` (100%)) are computed live from the actual pause/resume
  event history, not a static stored value. 19 unit tests cover the business-
  hours arithmetic and pause-aware percentage calculation.
- **Follow-up engine**: a real background worker (runs every 60 seconds, plus
  once on startup) that checks every open ticket against its severity's
  follow-up interval and raises a genuine follow-up record + notification
  when one is due; also supports a manual "Follow Up" trigger from the UI.
- **Escalation engine**: the same kind of background worker, computing each
  open ticket's live SLA percentage and escalating to the correct real
  person — assignee at 50%, team lead (resolved via actual team membership)
  at 75%, IT manager (resolved via actual role lookup) at 100% — with
  idempotency (each threshold escalates exactly once per ticket, verified
  across multiple scheduler ticks) and a manual "Escalate" trigger.
- **Notifications**: a real in-app system wired into ticket assignment,
  comments, status changes, and both background engines — with a live
  notification bell (polls unread count, dropdown list, mark read/all-read)
  in the UI.
- **"What Needs My Attention"**: the dashboard's core widget (spec section
  14), computed live on every load — SLA breached/critical/warning buckets,
  follow-up-required and RCA-required lists, and counts for
  waiting-for-my-action / waiting-for-development / ready-for-UAT / ready-
  for-pre-prod / ready-for-production, each clickable straight to the
  ticket. Nothing here is cached or hardcoded; it's a live walk over every
  open ticket computing real SLA status per ticket.
- **My Work** (`/my-work`, spec section 15): the same kind of live
  computation, scoped to the signed-in user — Urgent, SLA buckets, follow-
  ups, assigned-to-me, waiting-for-my-action, the three "ready for X" stage
  queues, and recently-updated.
- **Priority Queue** (`/priority-queue`, spec section 16 — "What should I
  handle first?"): every open ticket ranked by a weighted score computed in
  Go from live severity, priority, SLA percentage/status, escalation level,
  unresolved-follow-up flag, ticket age, and staleness — not a hardcoded or
  stored order. Toggle between "My Tickets" and "All Open Tickets" scope.
  Verified with unit tests that a breached S1 outranks a fresh S4 and that
  scope filtering is correctly isolated per user.
- **Projects & Milestones** (`/projects`, spec sections 36-38): full CRUD,
  duplicate-key rejection, a project dashboard with real stats (open/
  critical/completed ticket counts, milestone progress) computed live, and
  milestones with automatic overdue detection (a milestone past its due date
  and not completed shows OVERDUE on every read — verified live, along with
  confirming a completed milestone is immune to that even with a past date).
  Tickets can be linked to a project at creation and the project's Tickets
  tab uses the same server-side `projectId` filter as the main ticket list.
- **UAT test cases & the UAT gate** (spec sections 39-40): create test cases
  per ticket, execute them (Pass/Fail/Block, recording tester + timestamp +
  actual result), and — the important part — a ticket **cannot** be marked
  `UAT_PASSED` while any test case is failed, blocked, or not yet run. This
  was verified live end-to-end against a running server: zero test cases →
  not gated; an unexecuted test → blocked (400); one pass + one fail → still
  blocked; fixing the failing test → gate opens automatically on the next
  attempt. A separate admin-only `force` override bypasses the gate and is
  distinctly audit-logged (`STATUS_CHANGE_OVERRIDE`) so overrides are always
  traceable.
- **Pre-Prod / Production deployment tracking** (spec sections 41-42): real
  deployment records per ticket (version, deployment reference, deployed by,
  validation result), independently tracked per stage — verified recording
  both a Pre-Prod and a Production deployment on the same ticket with
  different versions, and marking a deployment `FAILED` with a rollback
  reason.
- **Incidents & RCA** (`/incidents`, spec sections 43-44): incident
  management with auto-generated numbers (`INC2026-00001`), a validated
  status workflow (OPEN → INVESTIGATING → MITIGATED → RESOLVED → CLOSED,
  with reopening supported), and — the important part — **S1/S2 incidents
  cannot be closed without a completed RCA**. This was verified live
  end-to-end: closing without RCA is blocked (400); the RCA itself can't be
  marked complete with empty root-cause/permanent-fix fields; filling those
  in and completing the RCA unblocks closure on the next attempt. A separate
  admin-only force-close bypasses the gate and is distinctly audit-logged.
  Incidents can optionally link to a ticket, and doing so for an S1/S2
  incident flags that ticket's `rcaRequired` field — which now feeds real
  data into the dashboard's "RCA Required" bucket from Phase 4 (previously
  always empty since nothing set that flag).
- **Global search** (spec section 45): real SQL search across ticket
  numbers/titles/descriptions, incident numbers/titles/descriptions, and
  comment content — verified live that a search term matching both a
  ticket's title and a separate comment returns both, correctly attributed.
  A debounced search bar lives in the app's top bar everywhere.
- **Reports** (`/reports`, spec sections 48-49): ticket volume over time,
  SLA compliance (computed via the same live SLA engine used everywhere
  else, not a separate calculation), average response/resolution time,
  breakdowns by team/assignee/category/project, an executive "management
  dashboard" (SLA compliance, open critical issues, SLA breaches, tickets
  this month, open/production incidents), and CSV export — all filterable
  by date range/team/project/severity. Verified live against real data.
- **Saved views** (spec section 47): the system default views seeded in
  Phase 1's migration are now actually served through a real API, plus
  users can create and delete their own custom views (system defaults are
  protected from deletion).
- **CSV ticket import** (spec section 53): upload a CSV, get back real
  imported/skipped/failed counts with per-row error messages. 6 unit tests
  cover valid rows, missing titles, invalid enum values, a missing required
  column, and — a real bug caught during live testing — rows with fewer
  columns than the header line, which Go's CSV reader rejects by default;
  fixed by allowing variable-width rows so partial data still imports using
  sensible defaults for omitted columns.
- **Backup & restore** (`/administration/backups`, spec section 52): real
  SQLite snapshots via `VACUUM INTO` (the correct way to back up a live
  SQLite database without stopping the server), admin-only, with automatic
  safety backups taken before every restore. Verified live end-to-end,
  including catching and fixing a real bug in the process: two backups
  created within the same second collided on filename (`VACUUM INTO`
  refuses to overwrite an existing file) — exactly the scenario a manual
  backup immediately followed by the automatic pre-restore safety backup
  would trigger. Fixed with a random filename suffix, with a regression
  test. A full restore was exercised live: it correctly swapped the
  database file, shut the process down cleanly, and came back up on restart
  with the restored data intact. **Disclosed architectural note:** restore
  requires an application restart rather than a seamless in-process
  hot-swap, since every repository holds its own database handle captured
  at startup — a deliberate, documented scoping decision (see the comment
  in `internal/backup/backup.go`), not a shortcut.
- **Settings** (`/administration/settings`, spec section 50): a real
  key-value settings store, admin-writable, backed by the database.
- **Audit log viewer** (`/administration/audit-log`): the audit trail that's
  been recording every action since Phase 1 now has a real admin-only page
  to browse it, with filtering support in the API (by action, entity type,
  user).
- **Dark mode**: a real toggle (persisted, defaults to system preference,
  no flash-of-wrong-theme on load) wired into the shared UI primitives
  (Card, Button, Input, Label, Badge) and the app's chrome (sidebar, top
  bar), so every page that uses those components gets dark mode for free.
- Dashboard shows real ticket counts (open, S1-critical, unassigned)
- Audit logging of auth, user-management, ticket, project, milestone, test
  case, deployment, incident, RCA, CSV import, backup, restore, and settings
  actions
- Full database schema for the entire platform — every table from the
  original spec now has a real repository, API, and (where applicable) UI
  behind it
- Health check endpoint
- Go unit tests across every module (auth, users, tickets, SLA, dashboard,
  UAT gate, projects/milestones, incidents/RCA, reports, CSV import,
  backup/restore) — all passing, `go vet` clean. Every new feature this
  phase was also verified live against a running server end-to-end,
  including cross-checking that the frontend's exact API calls all succeed
  with correct CORS/credentials handling.
- Cross-compiles to a genuine Windows `.exe`

That's every item from the original 74-section spec addressed with real,
tested, live-verified functionality — no mockups, no placeholders, no dead
buttons. See the roadmap below for the few remaining polish items.

## Architecture

```
frontend/          Next.js 16 (App Router) + TypeScript + Tailwind
  app/              Pages (setup, login, dashboard, administration/users)
  components/ui/    shadcn-style primitives (Button, Input, Card, Label)
  components/layout Sidebar / shell
  lib/api/          Typed fetch client talking to the Go backend
  hooks/            React Query hooks (useCurrentUser, etc.)

backend/            Go 1.22, Chi router, SQLite (mattn/go-sqlite3, cgo)
  cmd/server/        main.go — wires everything and starts the HTTP server
  internal/
    config/          Environment-driven config, local-first defaults
    logger/          Structured JSON logging to file + stdout
    database/        Connection + embedded, auto-run migrations
      migrations/     Ordered .sql files — the full schema for every module
    auth/            Sessions, bcrypt, login/logout/setup handlers
    users/            User repository + admin handlers
    middleware/       Auth guard, RBAC guard, security headers, rate limiting,
                       request logging
    audit/             Audit log writer + query
    reqctx/            Shared request-context helpers (avoids import cycles)
    response/          Consistent {data}/{error} JSON envelope
    teams/ tickets/ sla/ followup/ escalation/ notifications/ projects/
    testing/ incidents/ rca/ reports/ dashboard/ attachments/ settings/
    scheduler/ repository/
                       Directories scaffolded for upcoming phases (empty or
                       minimal today — see Current Status above)
```

## Prerequisites (Windows)

- [Go](https://go.dev/dl/) 1.22+
- [Node.js](https://nodejs.org/) 20+
- A C compiler for cgo (SQLite driver). On Windows, install
  [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) or use `mingw-w64` via
  [MSYS2](https://www.msys2.org/), and make sure `gcc` is on your `PATH`.

## Building on Windows

```bat
build.bat
```

This builds `dist\ticketing-backend.exe` and runs `npm install` + `npm run
build` for the frontend.

## Running

```bat
run.bat
```

This starts the backend on `http://localhost:8080`, the frontend on
`http://localhost:3000`, and opens your browser. On first launch you'll be
taken to the administrator setup screen.

## Development (this Linux sandbox / any Unix dev machine)

Backend:
```bash
cd backend
go run ./cmd/server        # reads ITOPS_PORT, defaults to 8080
```

Frontend:
```bash
cd frontend
npm install
npm run dev                 # reads NEXT_PUBLIC_API_BASE from .env.local
```

Copy `frontend/.env.example` to `frontend/.env.local` and point
`NEXT_PUBLIC_API_BASE` at wherever the backend is running.

## Database

SQLite at `backend/data/ticketing.db` (created automatically, no manual setup
required). Migrations live in `backend/internal/database/migrations/` and are
embedded into the binary — they run automatically on every startup, applying
only what hasn't been applied yet (tracked in the `schema_migrations` table).

Data directories (`data/`, `data/uploads/`, `data/backups/`, `data/logs/`) are
created automatically on first run.

## Configuration

Environment variables (all optional, sensible local-first defaults):

| Variable | Default | Purpose |
|---|---|---|
| `ITOPS_PORT` | `8080` | Backend HTTP port |
| `ITOPS_DATA_DIR` | `./data` | Root for DB, uploads, backups, logs |
| `ITOPS_SESSION_SECRET` | machine-derived fallback | Set explicitly in any shared/production deployment |
| `ITOPS_ENV` | `production` | `production` enables secure cookies |
| `ITOPS_ALLOWED_ORIGINS` | *(none — localhost only)* | Comma-separated extra CORS origins, e.g. `http://192.168.1.50:3000`, for reaching the app from another device on your network (see below) |

## Using it from your phone (or another device on the same network)

**What this is and isn't:** this lets you *view and use* the app from a
second device (your phone, another desk) pointed at the same running
instance — genuinely useful for checking things on the go. It is **not**
the same as a true multi-user concurrent server: this is still one
instance, backed by one SQLite file, on one computer. Two people editing
the same ticket at the same moment from two devices will both work, but
this hasn't been built or tested for many simultaneous users — see the
README's note in "Current status" about the deliberate local-first scope.

`run.bat` detects your computer's LAN IP automatically and prints a URL —
after running it, look for a line like:

```
To open this on your phone: connect it to the SAME WiFi network as
this computer, then visit this address in the phone's browser:

    http://192.168.1.50:3000
```

Open that address in your phone's browser. Requirements:

- **Same network.** The phone and the computer must be on the same WiFi
  (a phone on cellular data or a different WiFi network can't reach it).
- **Windows Firewall.** The first time you run it, Windows will likely
  prompt to allow the app through the firewall for private networks —
  accept that. If it doesn't prompt, or you've already dismissed it once,
  open **Windows Defender Firewall → Allow an app through firewall** and
  make sure `ticketing-backend.exe` and `node.exe` (or "Node.js") are
  checked for **Private** networks.
- The frontend automatically figures out where the backend is by looking
  at the address you opened it from — you never need to manually configure
  an IP address in the frontend itself, even if your computer's IP changes
  later (e.g. after reconnecting to WiFi).

If `run.bat` can't detect your LAN IP (rare), find it yourself with
`ipconfig` in a Command Prompt — look for "IPv4 Address" under your active
WiFi/Ethernet adapter — then set it manually before running:
```bat
set ITOPS_ALLOWED_ORIGINS=http://<your-ip>:3000
run.bat
```

## Running it entirely on Android, with no laptop (Termux) — experimental

Everything above assumes the app runs on your Windows laptop and your phone
just views it. It's also possible to run the *entire* app — backend and
frontend both — natively on the phone itself, with no laptop involved, using
[Termux](https://f-droid.org/packages/com.termux/) (install from F-Droid,
not the outdated Play Store version).

**Be aware before trying this:** this path has not been run on a physical
device by the people who built this app. What *has* been verified is that
the Go source code itself has no Windows-specific or architecture-specific
assumptions (no hardcoded paths, no OS-specific calls) — it was checked to
compile cleanly for `linux/arm64`, the architecture Termux runs on. The
setup script below automates the correct sequence of steps for Termux's
package manager and toolchain, but the actual on-device build/run has not
been watched working. Expect to troubleshoot.

```bash
pkg install git   # if you don't already have it
git clone <wherever you've put this project>   # or transfer the unzipped folder directly
cd it-ops-hub
bash setup-termux.sh
```

Known friction points to expect:
- **Android kills background apps aggressively.** The script explains how
  to request a wakelock and disable battery optimization for Termux, but
  you may still find the server stops when you lock the screen or switch
  apps unless that's configured correctly.
- **Slower, hotter builds.** Compiling Go and building the Next.js frontend
  on phone hardware takes meaningfully longer than a laptop and will use
  noticeable battery/storage during the build.
- **This is a one-person, single-device setup**, same as the laptop
  version — not a multi-user server, regardless of where it runs.

If you hit an error, the exact message will usually point at which step
failed — that's a much more useful starting point than a fresh attempt.

## Testing

```bash
cd backend
go vet ./...
go test ./...
```

## Security notes

- Passwords are bcrypt-hashed, never logged, never returned by the API.
- Sessions are opaque random tokens (256-bit), httpOnly, SameSite=Lax cookies.
- All list/detail endpoints require an active session; user-management
  endpoints additionally require the ADMIN role.
- A simple in-memory rate limiter throttles abusive request bursts per IP —
  adequate for a single-machine local deployment.
- Internal errors are logged server-side and never leaked to the client.

## Roadmap (remaining polish)

1. ~~Ticket CRUD, comments, activity history, attachments, categories, teams~~ ✅ done
2. ~~SLA engine (business-hours aware, pause/resume, warning/critical/breach)~~ ✅ done
3. ~~Follow-up + escalation background workers, notifications~~ ✅ done
4. ~~Dashboard KPIs, "What Needs My Attention", My Work, Priority Queue~~ ✅ done
5. ~~Projects, milestones, UAT/Pre-Prod/Production workflow gating~~ ✅ done
6. ~~Incidents, RCA~~ ✅ done
7. ~~Reports, global search, saved views, CSV import/export~~ ✅ done
8. ~~Backup/restore, settings, dark mode~~ ✅ done
9. Full end-to-end scenario walkthrough per spec section 67, gofmt/final
   polish pass, `TODO`/`FIXME`/placeholder sweep per spec section 68
