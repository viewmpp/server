Comparing two versions of a plan
================================

`internal/diff` takes two contracts, the previous version and the current one,
and returns a report of what changed. It is the base of the compare tool and of
the living link that will follow. This file records how it decides, what it was
measured on, and what it does not claim.

How tasks are matched
---------------------

Tasks are matched by `id`, the unique ID. Microsoft Project's own Compare
Projects uses the same key.

The row ID cannot be used. Inserting one task renumbers every row below it.
Measured on the two pairs below:

| Pair | Tasks | Rows that changed number |
|---|---|---|
| hospital | 98 | 37 |
| large | 1651 | 815 |

Every one of those tasks was still matched correctly by unique ID.

If a contract repeats an `id`, its first occurrence is used.

How a reader gets there
-----------------------

The viewer has a **Compare** button. There is no separate `/compare` page:
someone with two files opens one and compares with the other. A page aimed at
search would serve a few dozen queries a month (Keyword Planner, November
2026), and it would be one more near-copy of a landing page.

1. **Where the second version comes from.**
   - An anonymous visitor's button opens the file picker straight away.
   - A signed-in user's button opens a menu: a file from the computer, or one
     of their saved plans.
     - The list comes from `GET /api/v1/projects`. Only signed-in users can
       call it, and it shows only their own plans, newest first, as long as the
       `/projects` page.
     - The plan already open is left out.
     - A saved plan is read through `GET /api/v1/projects/{id}` and uploads
       nothing.
   - A file from the computer is uploaded with `store=no`. That stops it from
     being saved to the account and spending the free quota. It is counted as
     `upload_for_compare`, not as an opened plan.
2. **The browser sends both contracts** to `POST /api/v1/compare` as
   `{"previous": ..., "current": ...}`.
   - The route is behind `fromApp` and the export limiter, under its own key.
   - Each contract goes through `contract.Decode` with every policy limit.
   - The body may hold two full contracts plus 1 KB.
   - `compare_run` is counted when the report is sent.
3. **The order is guessed from progress.** A plan's percent complete only grows
   between updates. In the hospital pair it went from 40% to 43%.
   - The browser takes each version's progress: the project summary row when
     there is exactly one root and it is a summary, otherwise the leaf percent
     weighted by duration.
   - The version with more progress is taken as the newer one, and the header
     says the order was guessed.
   - With equal progress, the plan already open is taken as the newer one.
   - "Swap versions" reverses the order and clears the "guessed" note.
   - The contract carries no save date. A plan whose progress was rolled back
     would be guessed wrong.

   Without the guess, a person who opened the older file read "finish one day
   earlier" where it had slipped. That happened in the first manual test.
4. **The report opens in a dialog** with a summary and four tabs: what changed,
   what moved, links, critical path.
   - Every value from the files is set as text, never as HTML.
   - Highlighting changes on the chart itself is left for a later version.

Free and Pro
------------

Reading the report is free for everyone, signed in or not. The pricing page
says reading stays free, and seeing the report once is what makes someone want
more of it.

**Downloading the report as a workbook is Pro.**

- It is the file a planner sends a client every week, so it belongs to someone
  who updates schedules regularly and has a budget.
- `POST /api/v1/compare/xlsx` takes the same body as the comparison and
  `previous_name` / `current_name` in the query.
- It is behind `fromApp` and requires an account.
- The handler itself refuses users without Pro with a 403.
- The workbook is built from the two contracts again, never from a report sent
  by the browser.

**Sheets:**

| Sheet | Contents |
|---|---|
| Summary | Both file names; project start and finish with the shift in calendar days; the counts; the calendar-days note |
| Changed | One row per added or removed task and per edited field, with before and now |
| Moved | Working tasks, then summary tasks marked as such; start and finish before and now as Excel dates; shifts in days as numbers |
| Links | Changed relations with both tasks' rows and names |
| Critical path | Tasks that became critical or stopped being critical |

**Who sees the Pro dialog:**

- An anonymous visitor sees it at once, with a link to sign up.
- A signed-in user's export is sent to the server, and the server decides.
  - On a 403 the browser shows the dialog. Its "Turn on Pro" posts the account
    page's early access form in a new tab, so the report stays open.
  - The next press downloads.
  - Early access is granted only to a confirmed email address. A user who has
    not confirmed theirs gets a "Confirm your email" link instead of "Turn on
    Pro". The button would only bring them back with a message to confirm
    first, which the browser test ran into.
  - The page never trusts its own idea of whether the user has Pro, which
    could be out of date.

What the report holds
---------------------

The report separates what the planner changed from what moved because of it.
One three-day duration change in the hospital pair moved the start of 61 tasks
and the finish of 67. A single list buries the edit under its consequences.

| Field | What it holds |
|---|---|
| `start`, `finish` | The project's own dates: `project.start` / `project.finish`, falling back to the project summary row |
| `added`, `removed` | Tasks that exist in only one version, summaries included |
| `edited` | Per task, whichever of name, parent, duration, percent complete and milestone flag changed |
| `links` | Relations added, removed, or changed in type or lag |
| `moved` | Working tasks whose start or finish moved, largest finish shift first |
| `entered_critical`, `left_critical` | Working tasks whose critical flag changed |
| `rollups` | Summary tasks whose dates moved |

Every task in the report carries its unique ID, its row ID in the version it is
read from, its name and its outline number. The row ID is what lets a reader
find it in Microsoft Project.

Rules, and why each exists
--------------------------

- **Summary tasks and the project row are never edits.** Their dates, duration
  and percent are computed from the tasks inside them. Inserting a task at 0%
  dropped the Foundation summary from 86% to 80%, and nobody touched it. A
  summary's name and parent can still be edits. Its date moves go to
  `rollups`.
- **The project row** is the only root, and only if it is a summary. A plan
  with one top-level working task has no project row.
- **Links of an added or removed task are not listed.** Deleting one milestone
  removed three relations, and they come and go with the task. A relation is
  listed only when both its tasks exist in both versions.
- **Durations and lags are compared in one unit.** Saving the same plan again
  turned a zero lag from `0 HOURS` into `0 DAYS`. Units are converted to
  minutes:
  - working units use Microsoft Project's defaults: 8 hours a day, 40 a week,
    20 days a month;
  - elapsed units use clock time;
  - working and elapsed units are never compared with each other;
  - zero equals zero in any unit, and no lag equals a zero lag.
- **Numbers are compared with a relative tolerance of 1e-9.** A value read back
  through JSON must not count as a change.
- **Date shifts are calendar minutes.** Saying "working days" would need the
  project calendar applied, which is not done. The interface shows the shift in
  days or hours and must say "calendar".

What the report does not claim
------------------------------

- **No causality.** The parser does not reschedule, so nothing proves which
  edit moved which task. The report puts edits and moves side by side and never
  says "because of". Tracing a moved task back to an edit through the links is
  a candidate for later, and it would be labelled as an inference.
- **Assignments, notes, baselines and calendars are not compared yet.**

How it was measured
-------------------

The pairs were made in Microsoft Project 2016+ through COM automation, from
copies, by scripts that keep a journal of every edit. Each edit was found by
unique ID and the file was saved with `FileSaveAs`.

- **hospital.** `Hospital Expansion Project.mpp`, 98 tasks. Edits:
  - one task inserted mid-plan;
  - a milestone deleted;
  - a duration changed from 7 to 10 days;
  - progress recorded;
  - a lag changed from SS+56h to SS+80h;
  - an FF link changed to FS;
  - a rename.

  The two versions are `internal/diff/testdata/hospital-v1.json` and
  `hospital-v2.json`, parsed by the real parser.
- **large.** `large.mpp`, 1651 tasks. Edits: 3 inserts, 5 deletes, 18 duration
  changes, 29 progress updates and a rename, spread over the whole plan.
  - The compare took 1 ms and the report is 11 KB.
  - Every journal entry was found, and nothing else was.
  - The files are too large for the repository and are not kept.

**Automation can lose an edit without an error.** Setting 50% complete on one
hospital task returned no error, but Microsoft Project did not keep it: the
saved file still reads 0%, checked by reopening it. A hidden dialog that the
automation suppressed most likely cancelled it. So a journal is only trusted
after the saved file is read back. The fixture test asserts what the file
holds, not what the script asked for.

Not verified
------------

- **Primavera `.xer`.** Its unique ID is a database record number, and whether
  it survives between two exports of the same project is unknown. No real pair
  exists yet. Until one does, comparing `.xer` files must be labelled as
  unverified.
- **Pairs not made by automation.** Both pairs come from scripted edits. A plan
  revised by a person over weeks, with tasks cut and pasted, is the next thing
  to test.
