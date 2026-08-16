Parser contract
===============

The neutral JSON that the Java sidecar produces (`POST /parse`) and Go consumes.
Version 1.

This is **schedule data**, not a description of a picture. One document feeds
several consumers, and none of them may depend on the others:

```
Java (MPXJ) ──► contract ──┬──► mapper.js ──► DHTMLX model ──► chart
                           ├──► detail panel
                           └──► XLSX export (and later a public API)
```

There is one rule: **the contract must not contain a single field that exists
only because DHTMLX finds it convenient.** Translating into the library's model
is `mapper.js`'s job, and it is the only thing that knows about it.

The reason is not aesthetic. The DHTMLX model physically cannot hold what the
product needs: it has `id`, `parent`, `text`, `start_date`, `duration`,
`progress`, `type` - while the detail panel needs notes, resources with
allocation, predecessors with link types and lags, baseline, WBS and the
critical flag. All of that would ride alongside as custom fields, producing a
hybrid rather than the library's model. Export and sharing read data, not a
picture.

Authority
---------

**This document is the authority. Both implementations obey it.**

Neither MPXJ nor DHTMLX dictates the contract. MPXJ is one possible producer:
when version 17 renames a getter, the mapping inside Java changes and the
contract stays as it is. Symmetrically on the other side: swapping the renderer
changes `mapper.js`, not the contract.

The shape of the Java DTOs follows from the same principle - it is dictated by
this document, not by whatever MPXJ finds convenient to hand out.

---

Four rules that silently corrupt data
--------------------------------------

Each of these breaks **without an error**: nothing crashes, the plan is simply
displayed wrong.

### 1. Dates are local, without a time zone, always with a time component

Format: `yyyy-MM-ddTHH:mm:ss`. No `Z`, no offset, no space instead of `T`.

MS Project stores a schedule in **project time**. There is no time zone in the
file - "9:00" means nine in the morning wherever the project lives. Attaching
UTC here is wrong: the whole chart shifts by several hours.

The ban on the short form follows from that. JavaScript reads these two strings
**differently**:

| String | How JS reads it |
|---|---|
| `2026-01-05T08:00:00` | local time |
| `2026-01-05` | UTC, i.e. shifted by the whole time zone |

Mixing them in one document is not allowed. That is why even where a time is
meaningless - calendar exceptions - the full form with `T00:00:00` is written.

A space instead of `T` (`2026-01-05 08:00`) is **not defined at all** by the
ECMAScript specification. V8 and SpiderMonkey parse it today, but that is their
goodwill, not a guarantee. Seconds are always written, even when zero.

### 2. Duration is a value plus units, never a bare number

```json
"duration": {"value": 7.0, "units": "DAYS"}
```

A "day" in MS Project is a **working** day, and its length is defined by the
calendar (usually 8 hours, but 6 and 12 also occur). Collapsing a duration into
minutes is irreversible: without the calendar you cannot expand it back. And we
compute nothing on principle - we are obliged to show exactly what the user sees
in MS Project.

`units` is the name of the `org.mpxj.TimeUnit` constant **verbatim**. There are
fourteen (verified against the MPXJ 16.5.0 javadoc):

```
MINUTES  HOURS  DAYS  WEEKS  MONTHS  YEARS  PERCENT
ELAPSED_MINUTES  ELAPSED_HOURS  ELAPSED_DAYS  ELAPSED_WEEKS
ELAPSED_MONTHS   ELAPSED_YEARS  ELAPSED_PERCENT
```

We introduce no abbreviations of our own (`"d"`, `"ed"`): that would be a
fourteen-row table to maintain on both sides for zero benefit. `ELAPSED_*` is
calendar time that bypasses the working calendar and must not be confused with
the ordinary kind. `PERCENT` shows up on lags: "start when the predecessor is
50% complete".

The same rule applies to the **lag of a relation**.

### 3. No parent is `null`, not `0`

`0` is a real `uniqueID`: MPXJ hands out the project summary row under exactly
that number. Using `0` to mean "no parent" is a DHTMLX convention, and it makes
task zero its own parent.

In the contract the root is `parent_id: null`. Substituting `0`, and deciding
whether to show the project row at all, live in `mapper.js`.

### 4. The order of `tasks` is significant

**The `tasks` array reproduces the display order of tasks in the file. The
consumer is obliged to preserve it.**

The tree is assembled from `parent_id`, but the order of siblings under one
parent is not determined by that, and there is nowhere to derive it from:
not from dates, not from `id`, not from WBS. In MS Project the user sees tasks
in a specific sequence - often arranged by hand, and carrying meaning.

Re-sorting at any stage (Java, Go, the mapper) produces a different plan. On a
manually sorted file the viewer will silently show something other than what MS
Project shows.

The same applies to `relations` and `resources`, where the cost of an error is
lower.

---

Strings: everything that came from the file is untrusted input
---------------------------------------------------------------

This applies to **every** string field without exception: `tasks[].name`,
`tasks[].notes`, `resources[].name`, `calendar.name`,
`calendar.exceptions[].name`, `project.name`.

All of them come from someone else's file, uploaded by a stranger, and all of
them end up in HTML.

**The producer** is obliged to emit correct JSON: escaped quotes, backslashes,
newlines, and control characters `< 0x20` in `\uXXXX` form. The content itself
is **not modified** - no truncation, no stripping of markup. The contract's job
is to carry across whatever is in the file.

**The consumer** is obliged to treat every string as hostile and to escape it
when writing into HTML. No field is safe by construction.

---

The file name is not in the contract
-------------------------------------

The parser receives **bytes** and returns a schedule. What the file was, what it
was called and where it came from are none of its business.

The file name belongs to Go: Go accepted the upload, Go enforces the limits, Go
stores the result and renders the page. Sending the name to the sidecar and
getting it back is a round trip for a field Go already has.

There is a separate reason not to do it through a header like `X-File-Name`:
**file names can be Cyrillic, and HTTP header values are ISO-8859-1 by
specification.** `виадук.mpp` arrives as garbage unless encoded per RFC 5987 -
that is manual encoding on both sides for data that had no reason to travel at
all. Exactly the silent corruption the rules above are written against.

The name itself remains an untrusted string and the **only remaining trace** of
the uploaded file once the `.mpp` is deleted. Length limits, sanitising and
escaping on output are Go's responsibility, along with all other input.

**It is also the primary source for the header, not a fallback.** What
`project.name` contains depends on the format:

- for `.mpp` - usually empty (`null` in all four real files in the corpus):
  MS Project does not write the project name there; it lives in the name of the
  root task;
- for MSPDI - it contains **the name the plan was once saved under** (`<Name>`),
  not the title of the plan. In the file exported from MS Project it was
  `mspdi.xml`, while the actual title sat next to it in `<Title>` and does not
  reach the contract.

Hence the rule for the consumer: **prefer the name of the uploaded file**, and
use `project.name` only when there is no such name. The internal name goes stale
when the file is renamed, and the user would see a foreign string instead of the
one they just dragged in.

Substituting the root task automatically would be inventing data, so the parser
returns `null` as it is, and the page receives the file name from Go separately
from the schedule.

---

Schema
------

```json
{
  "contract_version": 1,

  "project": {
    "name": "Тестовый план - Виадук",
    "start": "2026-01-12T09:00:00",
    "finish": "2026-02-20T09:00:00"
  },

  "calendar": {
    "name": "Проектный календарь",
    "non_working_weekdays": ["SUNDAY"],
    "exceptions": [
      {
        "from": "2026-01-07T00:00:00",
        "to": "2026-01-07T00:00:00",
        "working": false,
        "name": "Рождество"
      }
    ]
  },

  "resources": [
    {"id": 1, "name": "Иванов И.И."},
    {"id": 2, "name": "Петрова А.С."}
  ],

  "tasks": [
    {
      "id": 8,
      "parent_id": 6,
      "name": "REST API",
      "wbs": "2.1.2",
      "outline_number": "2.1.2",
      "outline_level": 3,
      "start": "2026-01-29T09:00:00",
      "finish": "2026-02-06T09:00:00",
      "duration": {"value": 7.0, "units": "DAYS"},
      "percent_complete": 35.0,
      "is_summary": false,
      "is_milestone": false,
      "is_critical": true,
      "notes": "Зависит от схемы БД.",
      "baseline": {
        "start": "2026-01-27T09:00:00",
        "finish": "2026-02-04T09:00:00"
      },
      "assignments": [
        {"resource_id": 2, "units": 100.0}
      ]
    }
  ],

  "relations": [
    {
      "id": 4,
      "predecessor_id": 7,
      "successor_id": 8,
      "type": "START_START",
      "lag": {"value": 2.0, "units": "DAYS"}
    }
  ]
}
```

The Cyrillic sample values are deliberate: encoding is one of the things the
fixtures exist to protect.

### Task fields

| Field | Type | MPXJ source | Nullable |
|---|---|---|---|
| `id` | int | `getUniqueID()` | no |
| `parent_id` | int | `getParentTaskUniqueID()` | **yes** - that is the root |
| `name` | string | `getName()` | no |
| `wbs` | string | `getWBS()` | yes |
| `outline_number` | string | `getOutlineNumber()` | yes |
| `outline_level` | int | `getOutlineLevel()` | yes |
| `start` | date | `getStart()` | yes |
| `finish` | date | `getFinish()` | yes |
| `duration` | duration | `getDuration()` | yes |
| `percent_complete` | number, **0..100** | `getPercentageComplete()` | no, `0.0` when absent |
| `is_summary` | bool | `getSummary()` | no |
| `is_milestone` | bool | `getMilestone()` | no |
| `is_critical` | bool | `getCritical()` - **read from the file** | no |
| `notes` | string | `getNotes()`, empty string → `null` | yes |
| `baseline` | object | `getBaselineStart()` / `getBaselineFinish()` | yes |
| `assignments` | array | `getResourceAssignments()` | no, may be empty |

`percent_complete` is stored as **0..100**, as in the file and as the user sees
it. Dividing by 100 is a DHTMLX requirement and is done by the mapper. Always a
fractional number (`35.0`, not `35`): MPXJ returns `Integer` from MSPDI and
`Double` from `.mpp`, while the fixture is compared byte for byte and requires a
stable type.

`wbs` and `outline_number` are **different things** and must not be collapsed:
WBS is user-editable and may be an arbitrary code, while the outline number is
positional and generated automatically. They often match, but not always.

`is_summary` and `is_milestone` are independent: a summary task of zero duration
can be a milestone at the same time.

### Relations

A flat array, not nested inside tasks: the detail panel needs both predecessors
and successors, and duplicating a relation in two tasks guarantees divergence.
The consumer builds the index in both directions.

| Field | Type | Source |
|---|---|---|
| `id` | int | sequential numbering by the exporter |
| `predecessor_id` | int | `relation.getPredecessorTask().getUniqueID()` |
| `successor_id` | int | the task the relation was read from |
| `type` | string | name of the `org.mpxj.RelationType` constant |
| `lag` | duration | `relation.getLag()` |

`RelationType` has exactly four constants (verified against the javadoc):
`FINISH_START`, `START_START`, `FINISH_FINISH`, `START_FINISH`.

Relations are read through `getPredecessors()`; there is no `getTargetTask()`
method - the pair is `getPredecessorTask()` and `getSuccessorTask()`.

**`id` is a service field and is not comparable across files.** A relation has
no identifier of its own in the file; the number is generated by traversal
order. When comparing two versions of a schedule (v1.1 backlog), **the identity
of a relation is the triple `predecessor_id` + `successor_id` + `type`**. Build
a differ on `id` and any edit to the file shifts the numbering, so it will report
"40 relations removed, 40 added" where nothing changed.

### Resources and assignments

| Field | Type | Source |
|---|---|---|
| `resources[].id` | int | `Resource.getUniqueID()` → `Integer` |
| `resources[].name` | string | `Resource.getName()` |
| `assignments[].resource_id` | int | `ResourceAssignment.getResourceUniqueID()` → `Integer` |
| `assignments[].units` | number | `ResourceAssignment.getUnits()` → `Number` |

`units` is the allocation percentage: `100.0` means full occupancy. MS Project
displays this as "Петрова А.С. [50%]".

An assignment whose resource does not resolve is skipped: in the spike they all
resolved, but that is not guaranteed in other people's files.

**Unnamed resources are not exported.** MPXJ returns a service row with
`uniqueID 0` and an empty name; across a corpus of 15 real `.mpp` files no
assignment refers to it (verified). There is nothing to display, so it is
dropped. Dangling references to a dropped resource are caught by a dedicated
test - if such a file turns up, the test fails rather than showing an empty row
in the resources panel.

### Calendar

`non_working_weekdays` uses `java.time.DayOfWeek` names (`MONDAY`…`SUNDAY`),
sourced from `ProjectCalendar.getCalendarDayType(day) == DayType.NON_WORKING`.
Converting to `0..6` for `Date.getDay()` is the mapper's job; browser numbering
must not appear in the contract.

Exceptions come from `getCalendarExceptions()`: `from`, `to` (equal to `from`
when absent), `working`, `name`.

### Deliberately absent

| Removed | Why |
|---|---|
| `type: "project" \| "task" \| "milestone"` | DHTMLX vocabulary; the file holds two independent flags |
| `open: true` | widget state, not data |
| `text`, `start_date`, `end_date` | DHTMLX field names |
| `data`, `links` | DHTMLX collection names |
| `progress` 0..1 | DHTMLX scale |
| `type: "0"` on a relation | DHTMLX code |
| `non_working_weekdays: [0, 6]` | `Date.getDay()` numbering from the browser |
| `level` | renamed to `outline_level` - not to be confused with tree depth |

---

Known simplifications
----------------------

Recorded here so they do not look unnoticed.

**Partially working days are lost.** `working` is a boolean, but MS Project
allows a Saturday from 9:00 to 13:00: a day that is both working and not fully
so. Such a day travels into the contract as fully working. For timeline shading
this is acceptable - we either shade a column or we do not, there are no
half-tones. It becomes unacceptable if a Calendar view appears (not before
v1.2) or if working time is calculated. At that point `working` turns into a
list of intervals, and that will be contract version 2.

**Only the default project calendar is taken.** Per-resource calendars and
alternative task calendars are ignored. For shading non-working days on a shared
timeline this is sufficient.

---

The fixture as a two-way conformance test
------------------------------------------

A document that both sides can quietly drift away from is a bad contract. So the
reference is not only described here but also exists as a file that **both sides
run in their tests**:

| Side | What it checks |
|---|---|
| Java | the export of a real `.mpp` matches the fixture |
| Go | the fixture decodes into structs without loss and without unknown fields |
| JS | `mapper.js` digests the fixture and produces a correct DHTMLX model |

While there is no Java sidecar in the loop, Go serves this same fixture to the
viewer as the `/parse` response - that way the whole chain (page → mapper →
DHTMLX → details → export) is assembled and debugged end to end without a single
line of Spring. The `POST /parse` wrapper arrives last and replaces exactly one
function in Go: reading a file becomes an HTTP call returning the same JSON.

**One fixture is not enough.** At least two are needed; they cover different
things:

1. **A real `.mpp`** from the corpus - populated `is_critical`, dated summary
   tasks, genuine nesting depth. The corpus is in English.
2. **A Cyrillic plan** - encoding, notes and resource names in Cyrillic, all
   four relation types, a calendar exception, a baseline. This is synthetic
   MSPDI: we cannot write `.mpp`, since MPXJ writes only
   MSPDI/MPX/JSON/Primavera/Planner/SDEF. To obtain a real `.mpp`, open the
   MSPDI in MS Project and re-save it.

**Unresolved: where the fixture physically lives.** The three repositories have
no common root. The options are a git submodule, a monorepo, or a copy with a
checksum check in the tests on both sides. A copy without a check will not do:
that is precisely the silent divergence the fixture was invented to prevent.
This requires a decision.

---

Traps the spike already fell into
----------------------------------

The `java/mpp-viewer` spike is being discarded, but its mistakes are worth more
than its code. None of them failed with an exception.

**Lag lost its units.** `duration.getDuration()` was taken as a bare number and
`getUnits()` was never asked. A lag of "2 hours" travelled as `2.0` and was read
downstream as 2 days. The helper was named `days(...)`, which cemented the wrong
assumption.

**Task duration was not exported at all.** The "Days" column showed
`task.duration`, which DHTMLX computes itself - from its own working-time
configuration, not from the project calendar. This is reasoning about the
library's mechanics, not a measurement: the discrepancy was never checked against
a real file.

**Date formats were mixed.** Tasks used `yyyy-MM-dd HH:mm`, calendar exceptions
`yyyy-MM-dd`. Exactly the pair JS reads in different time zones.

**WBS was substituted by the outline number** when WBS was empty - the
distinction vanished silently.

**Assignments were reduced to a list of names.** The resource identifier and the
allocation were lost.

**Hierarchy is positional.** The tree is assembled from
`parent_task_unique_id`, not from `outlineLevel` and not from position. When
generating files, create tasks depth-first - otherwise children silently attach
to the wrong parent.

### A trap found during implementation

**`UniversalProjectReader.read()` returns `null` on an unrecognised file rather
than throwing.** The `throws MPXJException` signature hints at exactly the
opposite, and the javadoc says nothing about `null`. Without an explicit check,
any garbage reaches the mapping and dies there with a `NullPointerException` -
turning a bad file into a 500 "we have an outage" instead of a 400 "bring a
different file".

Measured on six inputs (`ErrorCodeTest`): plain text, a JPEG, zeroes, a
truncated real `.mpp`, half of one, and a valid OLE2 header followed by
garbage - **all six return `null`**, none throws. Two conclusions follow:

- the library gives no way to tell "wrong format" from "damaged file"; inventing
  a distinction that is not visible would mean lying to the user;
- the "damaged" branch exists as a defence but is not reached by any input we
  could construct. It is not called dead - we simply could not get to it.

An empty body never reaches our code at all: Spring rejects it.

### A trap absent from the spike but present in Spring

It is easy to fix the wrong thing here. **Spring Boot 4.1 serialises with
Jackson 3** (`tools.jackson.core:jackson-databind:3.1.4`), not Jackson 2 -
verified against the dependency tree. Three consequences follow:

- `JavaTimeModule` does not need to be registered: in Jackson 3, `java.time`
  support is built into databind (`tools.jackson.databind.ext.javatime`).
- `Jackson2ObjectMapperBuilderCustomizer` **does not exist** - that class is in
  no jar on the dependency tree. The Boot 4.1 equivalent is
  `org.springframework.boot.jackson.autoconfigure.JsonMapperBuilderCustomizer`.
- MPXJ pulls in **its own** Jackson 2 (`com.fasterxml.jackson:2.21.4`), so both
  major versions sit on the classpath. Configuring the wrong `ObjectMapper` is
  easy: the code compiles and silently has no effect. The annotations are shared
  and remain in the old `com.fasterxml.jackson.annotation` package.

The real trap all of this is for: Boot fixes the format itself, but per the ISO
standard it **omits zero seconds** - producing `2026-01-05T08:00` instead of
`2026-01-05T08:00:00`. Rule 1 requires the full form always, so the format is
set explicitly and globally rather than field by field.

---

What has not been verified
---------------------------

### Corpus homogeneity is the main limitation

The corpus: 17 real `.mpp` files (mpp8/9/12/14 plus two large ones saved by
MS Project), most of the small ones from one library's test set; a synthetic
Cyrillic MSPDI written by MPXJ itself; and one MSPDI saved by MS Project 2016.

While the corpus consisted only of `.mpp`, any conclusion drawn on it read as a
conclusion about the format - when it was in fact a conclusion **about this
corpus**. The first file of different provenance falsified the claim about
`project.name` immediately.

The discriminator turned out not to be the format but the **producer of the
file**: MS Project writes computed fields into both `.mpp` and MSPDI; MPXJ,
when writing MSPDI, does not. The only fixture where summary tasks arrive
undated is the synthetic one.

This was then confirmed on **one and the same plan**. The synthetic 1650-task
MSPDI was opened in MS Project and re-saved as `.mpp`; the result is in the
corpus as `large.mpp` (current format) and `large2007.mpp`. Re-saving enriched
it - see `SameProjectAcrossFormatsTest`:

| | synthetic MSPDI | same plan re-saved as `.mpp` |
|---|---|---|
| tasks | 1650 | 1651 (MS Project adds the project summary row) |
| summary tasks without dates | 150 | **0** |
| tasks flagged critical | 0 | **1100** |
| `project.name` | filled | `null` |
| relations | 1350 | 1350, identical set |

Nothing about the format changed the data. What changed is that MS Project
computed the schedule and wrote the results down.

**Therefore each claim below states which files it was measured on.** When a
file of new provenance appears, re-check exactly those whose note does not cover
it.

### Claims and their basis

- **`ELAPSED_*` and `PERCENT` were never observed.** *Measured on: the whole
  corpus, including the MSPDI from MS Project.* Units are only `DAYS`. We carry
  units anyway: the cost is one field, the cost of the error is silently wrong
  dates.
- **Summary tasks are dated.** *Measured on: 15 `.mpp` + MSPDI from MS Project.*
  **Does not hold** on the synthetic MSPDI: 3 summary tasks out of 3 arrive with
  `start: null`. Hence `rollUp` stays in the backlog - it is needed for MSPDI
  from third-party tools, not from Project.
- **`is_critical` is populated.** *Measured on: 15 `.mpp` + MSPDI from MS
  Project.* The synthetic MSPDI has no critical tasks at all - the flag was never
  written into it.
- **Nesting goes up to four levels.** *Measured on: the whole corpus.* Maximum
  `outline_level` is 4 for `.mpp` and 3 for both MSPDI files. Deeper nesting was
  not observed.
- **The source file format (mpp8/9/12/14) is not included in the contract** - no
  verified MPXJ method that returns it was found. Nothing was invented.
- **The project summary row is present in the export.** *Measured on: real
  `.mpp`.* It is `id: 0`, `parent_id: null`, `outline_level: 0`,
  `is_summary: true`, and the other tasks reference it through `parent_id: 0`.
  This does not contradict the contract - `0` is a real `uniqueID`. Whether to
  show it or collapse it is `mapper.js`'s decision; it was never checked by eye
  against MS Project.
- **`project.name` depends on the format.** *Measured on: the whole corpus.*
  Empty for `.mpp`; for MSPDI it holds the file name as of the moment of saving.
  See the section on the file name above.
- **Verification by eye against MS Project has not been done** for any file:
  hierarchy and dates were checked only for internal consistency (a parent
  precedes its child, no dangling references). Project 2016 and 2021 files are
  absent from the corpus - it covers mpp8, mpp9, mpp12, mpp14.
- **A 1000+ task file goes through the contract.** *Measured on: `large.mpp`,
  1651 tasks, 2.62 MB.* Parsed in 928 ms (533 ms for the mpp2007 variant, 742 ms
  for the same plan as MSPDI). Units are `DAYS` throughout, nesting depth 2, all
  dates in full form. `large.json` is a fixture, so the size case is covered on
  every run.
- **Size on disk, same plan, three encodings:** MSPDI 3.97 MB, `.mpp` 2.62 MB,
  mpp2007 2.45 MB - roughly 2.5 KB and 1.6 KB per task respectively. MSPDI is
  about 1.5x the `.mpp`, not an order of magnitude. This is what upload limits
  should be reasoned from: 10 MB is roughly 6000 tasks as `.mpp` and 4000 as
  MSPDI.

---

Decisions taken
----------------

| Question                                 | Decision                                                                                                                                                                                                                       |
|------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Rolling up summary task dates (`rollUp`) | **Not in v0.1.** MS Project dates them itself (13 of 13 real files). Needed only for programmatically generated files - and then together with a `dates_derived` flag, so that computed values are not passed off as read ones |
| Where this document lives                | **One canonical copy** here, in the Go repository. The Java repository holds a link, not a duplicate                                                                                                                           |
| `work` in assignments                    | In v1.1, together with the Resource Sheet. As an object with units, per rule 2                                                                                                                                                 |
| Who declares the DTOs                    | DTOs are written alongside the MPXJ layer; the controller only returns them. Their shape is dictated by **this document**, not by the library                                                                                  |
| The uploaded file name                   | **Not in the contract.** It belongs to Go - see the section above                                                                                                                                                              |
