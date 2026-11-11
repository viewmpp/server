Measurement
===========

What the service counts, why, and the thresholds the roadmap decisions wait on.

Every event is a daily total: one row per day and name in `event_counts`, with
no address, account or file name attached. `events.Recorder` keeps the totals
in memory and flushes them every minute, so a restart loses at most the last
minute.

Events
------

### Counted by the server

| Name | When |
|---|---|
| `upload_ok` | A file was read and the plan sent to the browser |
| `upload_from:<page>` | The same upload, by the page it started on: `/`, a landing slug, `/projects`, or `other` |
| `upload_rejected` | The parser could not read the file |
| `upload_too_large` | The file was over the upload limit |
| `upload_interrupted` | The browser stopped sending before the whole file arrived |
| `export_xlsx` | A workbook was sent from `/api/v1/xlsx` (viewer and `/mpp-to-excel`) |
| `export_saved` | A workbook was sent from `/p/{id}/xlsx` |
| `export_too_large` | A plan was over the contract limits on export |
| `save_too_large` | A plan was over the contract limits on save |
| `save_limit` | A free account was full |
| `share_created` | A private plan got a link |
| `share_open` | A shared plan was opened by someone other than its owner |
| `share_limit` | A free account tried to share past its limit |
| `share_page_from_link` | `/share-a-project-plan` was reached from a shared plan |
| `example_open` | An example plan was opened |

### Sent by the browser

The viewer sends these to `POST /api/v1/signals?name=...`. The endpoint
counts only names on the list in `events.FromViewer`; anything else is a 404
and is not counted. It sits behind `fromApp` and the read limiter.

| Name | When |
|---|---|
| `wish_pdf:<viewer>` | The "PDF · soon" button was clicked |
| `wish_xlsx_report:<viewer>` | The "Excel report · soon" button was clicked |
| `export_asked` | The question after an export was shown |
| `export_purpose:<answer>` | It was answered: `share`, `import`, `analyze`, `print` or `other` |

`<viewer>` says where the button was pressed:

- `owner`: the owner's saved plan;
- `shared`: a shared plan opened by someone else;
- `file`: a file opened without saving, including `/mpp-to-excel`;
- `example`: an example plan.

The difference between `owner` and `shared` is the point: a paid feature for
senders and one for recipients lead to different plans.

### What the browser-side counts can and cannot say

- **One vote per browser.** A vote is sent on the first click of each button
  only, remembered in `mpp-wish-voted`. The dialog still opens on every click.
- **At most three asks per browser.** The question opens as a dialog after an
  export, at most once per page load.
  - An answer ends it for good.
  - A skip (the Skip link or Esc) means "not now": it is asked again on a later
    page load, until the third skip.
  - `mpp-export-asked` holds `answered` or the number of skips.
- **Skip is visible on purpose.** Without a visible way out, some people pick
  any answer just to close the dialog, and that spoils the data.
- **Browsers, not people.** Two computers count twice, and a cleared browser
  counts again.
- **`export_asked` counts showings, not browsers.** One browser can add up to
  three, and a tab closed with the dialog open adds one more without using up
  a skip.
  - `export_purpose:* / export_asked` is the answer rate per showing.
  - The sum of `export_purpose:*` is the number of browsers that answered,
    since each answers once. That sum is what the thresholds below are about.
- **Blocked storage forgets everything.** In a browser with blocked storage,
  each page load can vote again, and the question comes back after every page
  load however it was answered.

Fake doors
----------

The two "soon" buttons are fake doors: the features do not exist. A click
opens a dialog that says so and counts as a vote. The label says "soon" up
front, so nobody clicks expecting a download.

- **What is being tested.** Only whether the feature is wanted at all. The
  buttons deliberately do not say Pro: whether a PDF can be paid for depends
  on the `/mpp-to-pdf` question in landing-strategy §9, and that is decided
  only if the demand is there.
- **When they come out.** Remove a button once its decision is made, either
  way. A door left up for months stops being honest.

Thresholds
----------

The same thresholds apply separately to each button's votes (all `<viewer>`
values together) and to the answers to the question after an export.

| Votes or answers | What to do |
|---|---|
| 10–19 | Look for patterns, build nothing large |
| 20–29 | Check the popular scenarios in conversations with users |
| 30–49 | With clear interest, build a small MVP |
| 50–99 | With confirmed demand, invest in the full feature |
| 100+ | Use the distribution of answers for more confident prioritisation |

These are practical guides, not statistical guarantees.

Reading the counts
------------------

The last 30 days, one row per day and name:

```bash
docker compose exec server wget -q -O - http://127.0.0.1:6060/events
```

The thresholds are about totals, which can need more than 30 days. The table
keeps every day, so sum it in Postgres:

```bash
docker compose exec server-db sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "
  SELECT name, sum(total) FROM event_counts
  WHERE day >= '\''2026-11-12'\''
  GROUP BY name ORDER BY name"'
```

Set the date to the day the release that added the counts went live.
