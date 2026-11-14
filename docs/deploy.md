Deploy
======

One Ubuntu VPS running Docker Compose. Caddy terminates TLS and is the only
container that publishes ports. Deploys are `git pull` plus `docker compose up`
on the host - no registry, no CI, no artifacts shipped from your machine.

Shape
-----

Caddy is the only container that publishes ports. It terminates TLS on 80 and 443
and proxies to `server:4000`. The server talks to `parser:8080` and to
`server-db:5432`, both of which live on the compose network with no `ports:` entry
at all, so there is nothing to reach even from the host's own loopback.

Files
-----

```
docker-compose.yaml         base: postgres, migrate, parser, server
docker-compose.dev.yaml     override: loopback ports for host development
docker-compose.prod.yaml    override: caddy
remote/production/Caddyfile mounted read-only into the caddy container
remote/setup/init.sh        one-time provisioning of a fresh server
remote/production/mpp-backup.service   nightly database backup
remote/production/mpp-backup.timer     schedule for it
.env.example                template for the .env that lives on the host
```

Production is the base file plus the prod override. There is no parallel
definition to keep in sync.

Why the dev override exists
---------------------------

Compose **appends** `ports` when it merges an override; it cannot remove them.
Putting loopback ports in the base file would mean production published them
too. So the base file carries none, and `docker-compose.dev.yaml` adds them back
for local work.

The practical consequence: **plain `docker compose up` gives you a stack you
cannot reach.** Local commands need both files.

Prerequisites
-------------

- An Ubuntu 24.04 VPS, **4 GB RAM minimum**. The parser is capped at 1536 MB and
  its image builds Gradle during `docker compose build`.
- A domain with an **A record already pointing at the VPS**. Caddy requests a
  certificate as soon as it starts; if DNS has not propagated, ACME fails.
- Ports 80 and 443 reachable. Nothing else needs to be.

First deploy
------------

Everything happens on the server. Nothing is copied up from your machine.

**1. Get on the box and clone both repositories.** The provisioning script lives in
this repository, so the clone comes first. GitHub needs a key it will accept, either
a deploy key or your own forwarded over SSH.

```bash
ssh root@<host-ip>
mkdir /viewmpp && cd /viewmpp
git clone git@github.com:<owner>/server.git
git clone git@github.com:<owner>/parser.git
```

The parser build context is `../parser`, so the two clones have to sit next to each
other under the same parent. Any other layout breaks the build with a confusing
error about a missing context.

**2. Provision.** The script asks for a password for the account it creates:

```bash
bash server/remote/setup/init.sh
```

It makes the `dzenthai` account, puts it in the `sudo` and `docker` groups,
installs Docker from get.docker.com, and hands `/viewmpp` over to that account. It
installs nothing else and clones nothing.

**3. Write the environment file.** Compose reads it for interpolation and hands it
to the server container, so without it step 4 stops before it starts anything:

```bash
cd /viewmpp/server
cp .env.example .env
nano .env
```

Fill in `POSTGRES_*`, `RESEND_API_KEY` and `SECRET_KEY`. The server refuses to start
in prod without `SECRET_KEY`, and it wants at least 32 characters out of a random
generator. URL-encode the database password if it contains `@ : / ? # &`, because
compose drops it into the DSN as a plain string.

**4. Bring it up.**

```bash
docker compose -f docker-compose.yaml -f docker-compose.prod.yaml up -d --build
```

**5. Verify.**

```bash
curl -fsS https://viewmpp.com/api/v1/healthcheck
# {"status":"OK","database":"OK","parser":"OK","env":"prod","version":"<sha>"}
```

Three separate facts, and the distinction is the point.

`status` is the process itself. It says `OK` whenever the endpoint answers at
all, and the HTTP code is always 200. That is what the container probe in the
`Dockerfile` reads, and it is deliberate: Caddy waits on the server being
healthy, so a sick dependency must not take the whole site down with it.

`database` and `parser` are the two things behind it, each probed with a two
second budget. Either one can be `DOWN` while the site keeps working in part -
without the database an anonymous visitor still uploads and reads a plan, and
without the parser every page that needs no parsing still serves. Alerting
belongs on these two fields, not on the HTTP code.

`parser: DOWN` is also the quickest proof that `PARSER_URL` is composed
correctly, because a wrong base sends the probe to a path that does not exist.

The verdict is cached for five seconds. The endpoint is public and unthrottled,
and without that cache every request would cost one database ping and one
outbound call to the sidecar. Nothing polls it faster than that anyway: the
container probe runs every 30 seconds.

Updating
--------

Both repositories are pulled and the stack is rebuilt in place:

```bash
cd /viewmpp/server && git pull
cd ../parser && git pull
cd ../server
docker compose -f docker-compose.yaml -f docker-compose.prod.yaml up -d --build
```

`--build` is not optional. The server image is built locally and tagged
`server:local`, so a plain `up -d` reuses the existing layers and quietly keeps
running the old binary.

**When the contract gains a field, the server goes first.** The server decodes
contracts with unknown fields refused, so a parser that already sends a new field
to an old server breaks every upload until the server catches up. Release the
server with the parser left as it is, then the parser:

```bash
docker compose -f docker-compose.yaml -f docker-compose.prod.yaml up -d --build --no-deps server
docker compose -f docker-compose.yaml -f docker-compose.prod.yaml up -d --build parser
```

That order applies to `project.status_date`, added with the lookahead.

Migrations run on the way up, and the server waits for them. Deploys are not
zero-downtime: `stop_grace_period: 40s` gives the old container room to finish
any verification email already in flight before Docker escalates to SIGKILL.

Since the diagnostics server was added, the surest proof that the new binary is
live is asking it - see below.

Caddy has no build stage, so a `Caddyfile` change on its own only needs the
container recreated:

```bash
docker compose -f docker-compose.yaml -f docker-compose.prod.yaml up -d caddy
```

Diagnostics
-----------

The server publishes `expvar` on a second listener, separate from the site:

```bash
docker compose exec server wget -q -O - http://127.0.0.1:6060/debug/vars
```

The scheme is not decoration. The image is Alpine, so `wget` is the BusyBox
build, and without `http://` it does not treat the argument as a URL at all -
the failures look unrelated to the real problem, `Permission denied` on a
nonexistent file or a refused connection.

It listens on `127.0.0.1` inside the container, which is deliberate and has one
consequence worth knowing: the port cannot be published to the host at all,
because Docker forwards a published port to the container's external interface
rather than to its loopback. `docker compose exec` is the only way in, and
`Validate` refuses to start production on any address that is not loopback.

Nothing guards the endpoint itself, so anything published there is readable by
anyone who can reach it. `/debug/vars` serves `cmdline` and `memstats`, neither
of them sensitive - secrets arrive through the environment, not as flags.
`/events` serves the daily usage counts of the last 30 days. They are business
data, which is exactly why the listener must stay on loopback. What each count
means is in [measurement.md](measurement.md):

```bash
docker compose exec server wget -q -O - http://127.0.0.1:6060/events
```

Local development
-----------------

```bash
docker compose -f docker-compose.yaml -f docker-compose.dev.yaml up -d
```

That publishes Postgres on `127.0.0.1:5432`, the parser on `127.0.0.1:8080` and
the server on `127.0.0.1:4000`.

To iterate on Go without rebuilding an image, start only the backing services
and run the binary on the host - `PARSER_URL` already defaults to
`http://localhost:8080`:

```bash
docker compose -f docker-compose.yaml -f docker-compose.dev.yaml up -d server-db migrate parser
go run ./cmd/web
```

Set `COMPOSE_FILE=docker-compose.yaml:docker-compose.dev.yaml` in `.envrc` and
plain `docker compose up` picks both files up.

Environment
-----------

`.env` on the host feeds two things: compose interpolation, and the `server`
container via `env_file`.

The `parser` container has no `env_file` and receives only `ENV`. It never sees
the database credentials or the Resend key - it is the one process that opens
untrusted binary input, so it is the one that gets nothing worth stealing.

| Variable | Value in production | Notes |
|---|---|---|
| `ENV` | `prod` | Turns on the `Secure` cookie flag and the `BASE_URL` guard |
| `BASE_URL` | `https://viewmpp.com` | **Must be https and not localhost.** The process refuses to start otherwise |
| `PROXIES` | `1` | One hop: Caddy. See the trap below |
| `DOMAIN` | `viewmpp.com` | Read by the Caddyfile as `{$DOMAIN}` |
| `ACME_EMAIL` | your address | Let's Encrypt account and expiry notices |
| `POSTGRES_USER` / `_PASSWORD` / `_DB` | | Compose assembles `DB_DSN` from these |
| `RESEND_API_KEY` | | No key means no verification or reset email |
| `RESEND_SENDER` | `noreply@viewmpp.com` | The domain must be verified in Resend first |
| `SECRET_KEY` | 32+ random characters | Signs every csrf token. The process refuses to start in prod without it |
| `EARLY_ACCESS_SEATS` | `100` | `0` closes early access |
| `LEMONSQUEEZY_API_KEY` | | Test or live key; it must match `LEMONSQUEEZY_TEST_MODE`. Billing is on only with all five `LEMONSQUEEZY_` values set; a partial set stops the process at start |
| `LEMONSQUEEZY_WEBHOOK_SECRET` | | The signing secret of the webhook pointed at `https://viewmpp.com/webhooks/lemonsqueezy` |
| `LEMONSQUEEZY_STORE_ID` | | Events from any other store are acknowledged and ignored |
| `LEMONSQUEEZY_VARIANT_MONTHLY` / `_YEARLY` | `2370696` / `2370717` | Events for any other variant are acknowledged and ignored |
| `LEMONSQUEEZY_TEST_MODE` | `true` until launch | `true` creates test checkouts and accepts only test events; `false` the reverse |
| `LEMONSQUEEZY_TEST_USERS` | your test account ids, comma separated | Only these accounts see the trial and the payment buttons while `LEMONSQUEEZY_TEST_MODE=true`; everyone else keeps early access. Ignored once live |
| `TRIAL_PERIOD` | unset | Defaults to `168h`. The length of the free trial once payments are offered |
| `DIAGNOSTIC_SERVER_ADDRESS` | unset | Defaults to `127.0.0.1:6060`. Must stay on loopback in prod, and off 4000, 5432 and 8080 |
| `LOG_LEVEL` | unset | Defaults to `info`. `debug` adds a line per request |

`PORT`, `DB_DSN` and `PARSER_URL` are set in `docker-compose.yaml` and override
anything in `.env` - they point at compose service names and must not be edited
per-host.

After changing `.env`:

```bash
docker compose -f docker-compose.yaml -f docker-compose.prod.yaml up -d
```

Migrations
----------

Migrations run on every `up`: the `migrate` service is gated on a healthy
database, and `server` waits for it to exit 0. No migration has been run by hand.

If a migration fails halfway, `migrate` marks the schema dirty and refuses to
run again until the version is forced with `force <version>`.

The `early_access` row in `quota_campaigns` is part of quota enforcement, not
ordinary campaign data. Seat claims lock it before counting active subscribers
and granting Pro. Save and share transactions instead lock the owner row before
counting, with access changes locking the project row second. Deleting the
campaign row makes seat claims fail closed.

Payments
--------

Pro is sold through Lemon Squeezy. Everything is off until all five
`LEMONSQUEEZY_` values are in `.env`; with them missing, the site behaves as
before payments existed.

### Test phase

1. In Lemon Squeezy test mode, create an API key and a webhook pointed at
   `https://viewmpp.com/webhooks/lemonsqueezy` with every `subscription_*`
   event. Its signing secret is `LEMONSQUEEZY_WEBHOOK_SECRET`.
2. Put the test key, the secret, the store id, both variant ids,
   `LEMONSQUEEZY_TEST_MODE=true` and your own account ids (`SELECT id FROM users WHERE email = ...`) in
   `LEMONSQUEEZY_TEST_USERS` into `.env`, then `up -d`. The log says
   `billing on test_mode=true`.
3. Only the listed accounts see the trial and the payment buttons. Pay with the
   test card `4242 4242 4242 4242`, any future date, any CVC. An account that
   already had early access has used its trial: test the trial on a new one.
4. Check after each step: the account page, `billing:` counts in `/events`, and
   `billing webhook` lines in `docker compose logs server`.

### Going live

1. Create the live API key and a live webhook (same URL, same events). Test and
   live webhooks have different secrets.
2. Remove what the test purchases left behind. Test subscriptions are already
   ignored once live, but the Pro they granted to the test accounts stays until
   its date:

   ```sql
   UPDATE users SET subscription_until = now()
   WHERE id IN (SELECT user_id FROM billing_subscriptions WHERE test_mode);
   DELETE FROM billing_subscriptions WHERE test_mode;
   ```

3. Put the live key and secret in `.env`, set `LEMONSQUEEZY_TEST_MODE=false`,
   then `up -d`. The log says `billing on test_mode=false`. From now on every
   visitor sees the trial and the payment buttons, and early access stops.
4. Open Manage subscription on a paying account. Lemon Squeezy refuses the
   customer portal with "This store has not been activated" until the store is
   approved, so it cannot be checked in the test phase; cancel a test
   subscription from Store, Subscriptions in the dashboard instead.

### What the code relies on

- The webhook is the only thing that changes a paid plan. It is checked by its
  `X-Signature`; an event for another store, another variant or the other mode
  is answered 200 and ignored. A failure is answered with an error so Lemon
  Squeezy retries it.
- Every checkout carries the site it was made on (`BASE_URL`), and the webhook
  ignores a checkout made elsewhere. There is one webhook and it points at
  production, so a test purchase from a laptop arrives there with a local
  account id that belongs to someone else in production.
- Deleting an account cancels its subscription first. If Lemon Squeezy cannot
  be reached, the account is not deleted and the user is asked to try again.
- The trial reminder runs hourly and claims a row before sending, so a failure
  loses that one email rather than sending it twice.

Backups
-------

The database is the only state that matters. A stored contract cannot be
regenerated - the uploaded `.mpp` is deleted immediately after parsing, so for a
signed-in user the row in Postgres is the only remaining copy.

A dump is taken every night. `remote/production/viewmpp-backup.timer` fires at
03:30 UTC with up to fifteen minutes of jitter and runs
`remote/production/backup.sh` as `dzenthai`. `Persistent=true`, so a night the
server spent switched off is caught on the next boot rather than skipped.

The script runs `pg_dump` **inside** the `server-db` container and takes the
credentials from that container's own environment, so no password lives in the
script, the process list or the logs. The dump is written in the custom format,
which is compressed and allows a single table to be restored on its own.

Three things happen in an order that matters. The file is written to `.part` and
renamed only on success, because a rename on one filesystem is atomic and a
half-written file that looks healthy is worse than no file. It is then read back
with `pg_restore --list`; a dump that cannot describe itself is deleted and the
run fails. Only after that are old copies removed - rotation last, so a script
that fails every night cannot quietly eat the archive.

Thirty copies are kept, counted rather than aged, in `/var/backups/viewmpp`
(mode 0700, owner `dzenthai`). The run refuses to start with less than a
gigabyte free: a backup must not be the thing that fills the disk and stops
Postgres accepting writes. Output goes to the journal - `journalctl -u
viewmpp-backup`.

**The off-site copy is manual.** From the workstation:

```bash
rsync -av dzenthai@viewmpp.com:/var/backups/viewmpp/ ~/dev/backups/viewmpp/
find ~/dev/backups/viewmpp -maxdepth 1 -type f -name 'viewmpp-*.dump' | sort | head -n -30 | xargs -r rm --
```

The second line is not housekeeping. The privacy page promises that a deleted
account leaves the backups within 30 days, and a plain `rsync` never removes
anything: without the pruning, every dump ever pulled stays on the workstation
and the promise is false. The pruning keeps the same 30 copies the server keeps,
counted locally rather than mirrored with `--delete`, so a compromised server
still cannot reach the off-site copies and empty them.

Pulled rather than pushed on purpose: the server holds no credentials for
anywhere else, so a compromised server cannot reach or destroy the copies. It is
a habit, not a schedule - weekly, and before anything risky. Hetzner's own image
backups cover a dead machine but not a deleted row, and they live in the same
account, so they are not the off-site copy.

Restoring has been rehearsed end to end, including dropping the production
database: stop `server` so nothing writes, drop and recreate the database,
`pg_restore` into it, check the row counts and `schema_migrations`, then bring
the stack up. `migrate` finds the version already recorded in the dump and does
nothing. The last step is not "the data is back" but "the site opens" - the
rehearsal ended with a restored database behind a stack that was still down.

Not done: the dumps are not encrypted, and nothing checks that the nightly run
actually happened. Both matter more once the database holds somebody else's
plans.

Logs
----

```bash
docker compose logs -f server
docker compose logs -f parser
docker compose -f docker-compose.yaml -f docker-compose.prod.yaml logs -f caddy
```

The server logs `r.URL.Path`, never the query string - share tokens travel in
URLs and must not reach the logs. If a change ever starts logging `RequestURI`,
that is a leak, not a formatting choice.

Traps
-----

**Caddy's volumes are load-bearing.** `caddy-data` holds the issued
certificates and the ACME account key. Without it Caddy re-issues on every
restart and hits the Let's Encrypt rate limit - 5 duplicate certificates per
week - after which the site serves TLS errors until the window rolls.

**`PROXIES` must match the number of proxies.** The resolver uses a
rightmost-trusted-count strategy over `X-Forwarded-For`. At `0` behind Caddy,
every request resolves to Caddy's address and all three rate limiters collapse
into one bucket for the entire internet. Set higher than the real hop count, a
client can spoof its own address by sending its own header. One Caddy, one hop,
`PROXIES=1`.

**Never add `ports:` to server, parser or server-db in the base file.** It is
what keeps them off the public interface. Docker publishes ports by writing
iptables rules that bypass ufw, so a host firewall will not save you from a
stray entry here.

**Upload limits live in two repositories.** Go caps free uploads at 10 MB and Pro
at 50 MB (`internal/user/user.go`); the sidecar's `parser.max-body-bytes` is
52428800 - the same 50 MB. Raising the paid tier above that fails at the sidecar
with no useful message. The Caddyfile sets `max_size 64MB` deliberately *above*
both, so an oversized upload gets the application's own JSON 413 rather than a
truncated stream.

**`PARSER_URL` is a base address, not an endpoint.** Go appends `/parse` and
`/healthcheck` itself. It is set in `docker-compose.yaml`, never in `.env`, so
it travels with `git pull` and needs no edit on the host - but an old value
ending in `/parse` left in a shell or an `.env` makes every upload a 404 against
`/parse/parse`, with no useful message. The healthcheck catches it: `parser`
reads `DOWN` while the sidecar is perfectly healthy.

**`RESEND_SENDER` on an unverified domain silently fails.**
`onboarding@resend.dev` works for testing only and cannot send to arbitrary
addresses.
