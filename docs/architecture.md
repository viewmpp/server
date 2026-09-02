Architecture and decisions
==========================

What the product is, how it is put together, and the facts that were expensive to
establish. Committed deliberately: losing it would mean re-deriving measurements
against real files.

Two sibling documents carry the rest. `contract.md` is the authority on the JSON
the Java sidecar produces and Go consumes. `deploy.md` covers the server, the
compose files and the traps in running them.

## Product

A web tool: the user drags in a `.mpp` (Microsoft Project) file and immediately sees a readable interactive Gantt chart - no installation, no registration. He can share a read-only link with colleagues who also have nothing installed.

The user is a **recipient of the file**, not a planner. He was sent a schedule, has no MS Project license, and needs to read it now.

## Hard boundary

This is a **viewer**. Not an editor, not a project-management system.

- No task editing, no dragging bars, no changing dates
- No schedule recalculation - if moving a task should shift its dependents, that's a scheduler, and we are not becoming one
- No team accounts, comments, assignments, notifications, or integrations

Any capability that lets the user modify the schedule violates this boundary. A proposal that
crosses it needs an alternative that stays inside.

## Architecture

A request goes from the browser to Go, out to the Java sidecar, and back. Go answers
the browser with the result and, if the user is signed in, also writes it to Postgres.
The sidecar sits on an internal network and nothing outside can reach it.

**Go** - public perimeter and the product itself: SSR landing page, file intake, limits, parsing calls, and - for signed-in users only - storage, share links, auth and subscriptions as packages inside the same binary.

**Java sidecar** - a dumb stateless converter. One endpoint, `POST /parse`: file in, normalized JSON out. No database, no secrets, no state, no internet access.

**Postgres** - storage for signed-in users. Never touched by anonymous traffic.

**Browser** - renders the chart.

### Two tiers, and why

**Anonymous - nothing is stored.** Upload, parse, and the contract goes straight back in the response body. The page renders from it. Refresh loses the plan; that is accepted, not a defect.

This protects the core promise: the user was sent a schedule, has no MS Project licence, and needs to read it **now**. Registration is friction in exactly the place where there must be none. It also means anonymous traffic costs CPU and nothing else - no disk, no TTL sweeps, no cleanup.

**Signed in - the parse is stored** and appears in the user's list of projects. It gets a stable URL the user can return to. By default the project is private: the URL exists, but access is refused to anyone but the owner.

**Subscribed** - may set `access` on a project (private / public / …) and upload larger files.

### What each state may do

Two independent axes get confused constantly: **email verification** and **subscription**. They gate different things, and neither substitutes for the other.

| | Anonymous | Free, unverified | Free, verified | Pro |
|---|---|---|---|---|
| Upload and view a plan | ✓ | ✓ | ✓ | ✓ |
| Save a project to the account | - | ✓ | ✓ | ✓ |
| Recover a forgotten password | - | **✗** | ✓ | ✓ |
| Subscribe | - | **✗** | ✓ | ✓ |
| Make a project public (share) | - | - | ✓ (2 at a time) | ✓ (unlimited) |
| Upload above 10 MB | - | - | - | ✓ (50 MB) |

**Verification is not a feature gate - it is an access-recovery gate.** The only thing an unverified free user actually loses is the ability to get back in after forgetting the password, because there is no proven address to send a reset to. Subscription is also blocked: selling to an address that may not exist has no way to deliver a receipt.

**Sharing always requires a verified address**, at any tier - links sent from an unproven mailbox are a reputation problem. Beyond that the free tier gets **2 public projects at a time**; Pro removes the cap. The limit counts projects currently public, not projects ever shared: stop sharing one and the slot returns. Free sharing exists because the link is the growth channel - the recipient meets the product with no barrier at all - and capping it at two is what leaves something to sell.

**Saving is deliberately free.** What is sold is the second party: a colleague with no MS Project opening the plan through a link. That is the recurring value and the reason the product exists; storing a file for its own owner is the hook that leads there, not the product.

Do not word user-facing messages as if verification unlocked sharing - it does not, and that wording has already been written and corrected once.

Both tiers run the same code path. The contract is returned in the response body in **both** cases; the only difference is one `if` that also writes it to Postgres. The frontend never branches - it always renders from the upload response, and a signed-in user additionally gets a URL. Two frontend paths would be two sets of bugs, one of them barely exercised.

Rules:

- The browser **never** calls the Java service directly. Two reasons: limits and paywall logic live in Go and must not be bypassable with curl; parsing an untrusted binary file is the most dangerous surface in the product, so the parser stays in an isolated box with nothing worth stealing.
- Parsing is **synchronous** - measured fast enough that no queue or async pipeline is needed. Don't introduce one without new measurements.
- The uploaded `.mpp` is deleted immediately after parsing. For a signed-in user the stored contract is then the only remaining copy - it cannot be regenerated. For an anonymous user nothing survives the request at all.
- Keep three data layers separate: neutral contract JSON from the parser → `mapper.js` → DHTMLX model. Java must **not** emit DHTMLX-shaped JSON - that couples the backend to one rendering library and turns a renderer swap into a rewrite.
- **Access is a column, not a URL property.** A share link alone is not the right to view: the server also checks `access` on the project. This makes sharing revocable - the earlier "the token *is* the capability" model could not take a link back once sent.
- Keys are still random tokens (`crypto/rand`, 16+ bytes, base64url), never sequential ids, so a private project cannot be found by enumeration even before the access check runs.
- **Never log the query string.** Tokens travel in URLs; `r.RequestURI` in a log line hands every project to whoever reads the logs. Log `r.URL.Path`.
- Contracts are stored gzipped. Measured on the corpus: a 1650-task plan is 782 KB of JSON and 47 KB gzipped - 16x, because the keys repeat.
- **Upload limits exist in two repositories and must move together.** Go caps free uploads at 10 MB and subscribed ones at 50 MB; the sidecar's `parser.max-body-bytes` is 50 MB. The same numbers are also written into page copy in several templates, so a change has more homes than it looks. Raising the paid tier above that silently fails at the sidecar. `Content-Length` is also a hint, not a guarantee - it is absent under chunked encoding - so `http.MaxBytesReader` stays as the backstop.

## Stack

- **Go standard library html/template** for SSR. SSR here does not mean "no JavaScript" - the server assembles the HTML, and the viewer page carries a JS widget inside it.
- **Java** (Java 8 compatible build) + **MPXJ** for parsing
- **DHTMLX Gantt Community 10.0.0** (MIT) for rendering
- **Postgres** for stored contracts and share links.
- **No npm, webpack, React, or Node.** The library is vendored as a UMD file in `static/` and embedded into the binary via `go:embed`. Keep the "one Go binary + one Java sidecar + Postgres in docker-compose" shape.
- Pin dependency versions.

### Why Postgres

At first there was no store at all: one Go binary and one Java sidecar. Anonymous
viewing still works exactly that way and nothing about it changed. Postgres came in
for the signed-in tier, because a project you can come back to, and a link a
colleague opens tomorrow, both have to outlive the request that created them.

Redis was the obvious alternative and lost on three counts.

Durability is the whole requirement here, and Redis is a cache by nature. Turning it
into a datastore means configuring AOF or RDB, and eviction under memory pressure can
still drop a key without an error surfacing anywhere. A saved project quietly
vanishing is the exact failure this product exists to avoid.

Blobs would sit in RAM. A large plan is 782 KB; multiply that by however many
projects people keep. Postgres puts them on disk and TOAST-compresses them without
being asked.

Users, projects and access levels are rows with foreign keys anyway. Picking Redis
for the blobs would have meant adding Postgres later for everything else, so two
dependencies instead of one.

Redis does win on TTL, which it has natively. In Postgres that is an `expires_at`
column, a filter and a cleanup job, about fifteen lines. A real cost, just not one
worth a second datastore.

### Schema notes

- `access` is `TEXT` with a `CHECK` constraint, not a Postgres `ENUM`. Extending an `ENUM` requires `ALTER TYPE`; a `CHECK` is an ordinary migration.
- A subscription tier name cannot express expiry. Whatever the tier column is called, entitlement is decided by `subscription_until > now()` - otherwise a background job has to flip a boolean and the moment of lapse is lost.
- Tier checks belong behind a method (`u.HasSubscription()`), not a string literal compared at each call site. A typo in `"free"` compiles.

## Verified facts - do not re-derive or contradict

Everything below was established by measurement against real files, not from documentation or
memory. Anything that contradicts it needs a fresh measurement first.

### MPXJ

- Maven groupId is still `net.sf.mpxj`, current version 16.5.0. Only the internal Java package was renamed to `org.mpxj.*`. There is no `org.mpxj` groupId on Maven Central.
- There is **no `.mpp` writer** - the binary format is read-only. Writers available: MSPDI, MPX, JSON, Primavera, Planner, SDEF. To produce a test `.mpp`, generate MSPDI XML and re-save it through MS Project.
- `UniversalProjectReader` detects the format itself. No per-version handling is needed - mpp8, mpp9, mpp12 and mpp14 files all open through the same code path.
- Relations: `relation.getPredecessorTask()` / `getSuccessorTask()`. There is no `targetTask` method. `Relation` is built through `Relation.Builder`.
- Dates are `java.time.LocalDateTime`, not `java.util.Date`.
- Build the task tree from `parent_task_unique_id`, not from `outlineLevel` and not from position. Hierarchy in MSPDI/MPP is positional: a level-2 task attaches to the nearest preceding level-1 task. When generating files, create tasks depth-first - otherwise children silently attach to the wrong parent with no error raised.
- MPXJ prints a Log4j "no logging provider" warning to stderr. Harmless; fixed by adding any log4j2 implementation.

### Data in real `.mpp` files

- `isCritical` comes from the file - MPXJ does not compute the critical path. **Do not implement critical path calculation.**
- Summary tasks already carry full start/finish dates, written by MS Project. **Do not write recursive date rollup.** It would only be needed if the product later accepts programmatically generated files (MSPDI from other tools, GanttProject exports) - keep that as a known backlog edge case.

### DHTMLX Gantt Community 10.0.0

- License is MIT, verified inside the package. GPL claims found online refer to older versions.
- Virtualization is real and `smart_rendering` is on by default in the Community edition - DOM node count stays constant regardless of task count.
- **Every bulk tree operation must go through `gantt.batchUpdate()`.** Each `close()` / `open()` triggers a full re-render, so a naive "collapse all" loop freezes the UI for seconds on large files.
- The left tree grid is built in, and `parent` exists in the data model.
- PRO workarounds are verified working in Community: critical path via `templates.task_class`; WBS via a normal `config.columns` column; non-working day shading via `templates.timeline_cell_class`; resources as a plain HTML table with no library involvement.
- **PRO is not needed.** The locked features are scheduling computation, not rendering. This product computes nothing - it reads results already stored in the file.
- Baseline: the public `gantt.addTaskLayer` is deleted in Community, but the underlying layers service is reachable through `gantt.$services.getService('layers')` and baseline bars render correctly with virtualization. This is an undocumented `$`-prefixed internal API. **The decision is deferred to v1.1 - do not implement it now.**


### Input formats

Verified by putting one real file of each kind through the running sidecar:
`.mpp`, Project `.xml` (MSPDI), `.mpx`, `.mpd` and Primavera `.xer` all parse.
`UniversalProjectReader` decides from the bytes, so the extension never reaches
any decision on the server; the `accept` attribute on the file input only filters
the picker dialog, and drag-and-drop bypasses it entirely.

**Localised MPX does not parse.** A German `.mpx` uses `;` as the field separator
and `,` as the decimal mark, and the reader defaults to the English locale, so the
file comes back as "damaged". MPX is a 1990s format and non-English variants exist
in the wild. The failure is indistinguishable from a genuinely corrupt file.

### Authoring a plan without MS Project

`contract.md` records that a synthetic MSPDI written by MPXJ arrives with no
critical path and undated summary tasks. That is a limit of **MPXJ's writer**, not
of the format or the reader: a hand-authored MSPDI carrying `<Critical>`,
`<PercentComplete>`, `<Summary>` and explicit summary dates is read back verbatim -
measured by comparing the XML against the resulting contract.

This is how the example plans in `internal/examples` are made: the schedule is
computed by a forward and backward pass, written out as MSPDI, and put through the
real sidecar. The XML sits next to each contract so a plan can be rebuilt.

## Scope

**v0.1 - this only:** drag-and-drop upload without registration; interactive Gantt (zoom by day/week/month/quarter, collapse and expand hierarchy levels, click a task to open a detail panel with dates, duration, percent complete, predecessors, resources and notes, dependency arrows for all four link types, critical path highlighting, non-working day shading, optional grid lines); left task table with search; accounts with email verification; saved projects for signed-in users; share links, gated by the rights matrix above; XLSX export. No payments, no editing.

Accounts are part of v0.1 and not optional: saving and sharing both hang off them. Viewing never requires one - anonymous upload, anonymous view of a shared link.

XLSX export and password recovery are built. Link TTL is not implemented and not decided -
revocation today is the `access` column, which is enough on its own.

**v1.1 backlog** - one at a time, only if traffic materializes: Resource Sheet (a plain table), Tracking Gantt with baseline, Timeline (a compact summary for forwarding), and comparison of two schedule versions.

**Never:** Network Diagram, Usage views, Form views, Rollup views, Team Planner, Relationship Diagram, Leveling Gantt. Calendar view not before v1.2. Anything that exists in MS Project for the purpose of editing is out of scope by default.

## Code rules

- No new dependencies without explicit approval - each one is a license, size and maintenance question.
- No frontend toolchain.
- Isolate the rendering library behind a thin interface so it can be replaced without a rewrite.
- Don't optimize without measuring. Don't build for hypothetical future scale.
- **No comments in code.** Not in Go, not in JS, not in templates. If a line needs explaining,
  rename it or split it until it does not; the reasoning belongs in these documents. Compiler
  directives such as `//go:embed` are the only exception.
- Acceptance checks on any change to parsing or rendering: Cyrillic text intact; hierarchy and dates match what MS Project shows; a 1000+ task file stays readable and doesn't kill the page; files saved by Project 2003, 2010, 2016 and 2021 all open.


## Upload flow

One request does the whole thing. Nothing is queued and nothing touches the disk.

1. `GET /` returns the page: a dropzone, and a chart that starts hidden.
2. The user drops a file, so JS now holds a `File` object.
3. `fetch POST /api/v1/upload` with the file bytes as the body.
4. Go applies the size limit to the request body.
5. Go streams those bytes on to `POST /parse` on the sidecar.
6. The sidecar answers with the contract JSON.
7. Go writes that JSON straight into its own response and is finished. The bytes
   were a local variable and the garbage collector takes them.
8. `res.json()` hands the contract to the browser.
9. `MppMapper.toModel()` converts it and `gantt.parse()` draws it.
10. The dropzone hides, the chart appears. The file name comes from `file.name` -
    the server never learns it.

For a signed-in user step 7 also writes the contract to Postgres, gzipped, and puts
the new project id in a header. That is the only branch in the whole flow, which is
why the frontend never has to know which tier it is talking to.
