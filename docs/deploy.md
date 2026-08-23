Deploy
======

One Ubuntu VPS. The Go binary and the parser jar run as systemd services, Caddy
terminates TLS, Postgres runs on the box. Deploys are `rsync` over SSH from your
machine - no registry, no CI, no Docker in production.

Shape
-----

```
internet ──► caddy :443 ──► api :4000 ──► parser 127.0.0.1:8080
             (systemd)      (systemd)     (systemd, loopback only)
                                │
                                └──► postgresql :5432
```

Three unprivileged accounts, not one:

- `mppviewer` owns the binary, reads `/etc/mppviewer.env`, and is the account you
  SSH in as.
- `parser` runs the sidecar. No shell, no sudo, no SSH key, and no access to the
  environment file. It is the only process that opens untrusted binary input, so
  it is the one that gets nothing worth stealing.
- `postgres` as installed.

Repository layout
-----------------

```
remote/
├── production/
│   ├── api.service      systemd unit for the Go binary
│   ├── parser.service   systemd unit for the Java sidecar
│   └── Caddyfile        production reverse proxy config
└── setup/
    └── 01.sh            one-time provisioning of a fresh server
```

Everything is driven from the Makefile. `make help` lists the targets.

Prerequisites
-------------

- An Ubuntu 24.04 VPS with **4 GB RAM minimum** - the parser is capped at 1536 MB
  and the JVM plus Postgres plus Caddy needs headroom above that.
- Both repositories cloned side by side. `make build/parser` builds `../parser`:

  ```
  mpp-viewer/
    server/     <- you deploy from here
    parser/
  ```

- Java 25 and Go 1.26 on **your** machine (the jar and the binary are both built
  locally and shipped as artifacts; the server compiles nothing).
- A domain with an **A record already pointing at the VPS**. Caddy requests a
  certificate the first time it loads the Caddyfile; if DNS has not propagated,
  ACME fails.

First deploy
------------

**1. Provision the server.** Copy the script up as root and run it:

```bash
rsync -P ./remote/setup/01.sh root@<host-ip>:~
ssh -t root@<host-ip> 'bash 01.sh'
```

It prompts for the domain, a Postgres password, and the Resend key and sender.
It then creates both service accounts, configures ufw (22/80/443 only) and
fail2ban, installs the `migrate` CLI, Postgres 17 from PGDG, Temurin 25, and
Caddy, writes `/etc/mppviewer.env`, and reboots.

**2. Set a password for the deploy user.** The script deletes it and forces a
change on first login:

```bash
ssh mppviewer@<host-ip>
```

**3. Point the local config at the box.** Two edits:

- `production_host_ip` at the top of the [Makefile](../Makefile)
- the domain on the first line of [remote/production/Caddyfile](../remote/production/Caddyfile)

**4. Ship it.**

```bash
make production/deploy/all
```

That builds both artifacts, rsyncs them up, runs migrations, installs and starts
both units, and reloads Caddy. The order matters and the target encodes it:
parser first, because `api.service` declares `Requires=parser.service` and will
refuse to start without it.

**5. Verify.**

```bash
curl -fsS https://your-domain.com/api/v1/healthcheck
# {"status":"OK","env":"prod","version":"<sha>"}
```

Deploying an update
-------------------

```bash
make production/deploy/api      # Go changed
make production/deploy/parser   # Java changed
make production/deploy/caddy    # Caddyfile changed
make production/deploy/all      # all three
```

`production/deploy/api` runs `migrate up` before restarting the unit, so schema
changes ride along with the code that needs them.

Deploys are not zero-downtime. `systemctl restart` stops the old process and
starts the new one; the gap is a second or two. `TimeoutStopSec=40` in
[api.service](../remote/production/api.service) gives the old process room to
drain its 30-second shutdown context and finish any background email send before
systemd escalates to SIGKILL.

Environment
-----------

Production configuration lives in `/etc/mppviewer.env`, owned `root:mppviewer`
with mode `0640`, loaded by `EnvironmentFile=` in the unit.

This deviates from the book, which puts the DSN in `/etc/environment`. That file
is world-readable, and this project has a Resend API key to protect, not just a
database password. Any account on the box could read it there.

Every setting in `internal/config` is a flag with an env fallback, so nothing
needs to be passed on the `ExecStart` line - the unit just runs the binary.

| Variable | Set by `01.sh` | Notes |
|---|---|---|
| `ENV` | `prod` | Turns on the `Secure` cookie flag and the `BASE_URL` guard |
| `BASE_URL` | `https://<domain>` | **Must be https and not localhost.** The process refuses to start otherwise |
| `PROXIES` | `1` | One hop: Caddy. See the trap below |
| `PORT` | `4000` | Change it and the Caddyfile upstream changes too |
| `DB_DSN` | assembled | Local socket, `sslmode=disable` - the traffic never leaves the box |
| `PARSER_URL` | `http://127.0.0.1:8080/parse` | |
| `RESEND_API_KEY` | prompted | No key means no verification or reset email |
| `RESEND_SENDER` | prompted | The domain must be verified in Resend first |
| `EARLY_ACCESS_SEATS` | `100` | `0` closes early access |

To change one: edit the file as root, then `sudo systemctl restart api`.

Migrations
----------

Run automatically by `make production/deploy/api`. To run one by hand:

```bash
make production/connect
source /etc/mppviewer.env
migrate -path ~/migrations -database "$DB_DSN" up
```

Roll one back with `down 1`. If a migration fails halfway, `migrate` marks the
schema dirty and refuses to run again until the version is forced:

```bash
migrate -path ~/migrations -database "$DB_DSN" force <version>
```

Rollback
--------

```bash
git checkout <previous-sha>
make production/deploy/api
```

If the rolled-back code predates a migration that already ran, roll the
migration back **first**. `migrate up` runs before the restart, so a
forward-only schema paired with backward code fails at query time, not at boot,
which is the harder failure to read.

Backups
-------

The database is the only state that matters. A stored contract cannot be
regenerated - the uploaded `.mpp` is deleted immediately after parsing, so for a
signed-in user the row in Postgres is the only remaining copy.

```bash
make production/backup      # writes ./backups/backup-<date>.sql.gz
```

Restore:

```bash
gunzip -c ./backups/backup-2026-01-01-1200.sql.gz | \
  ssh mppviewer@<host-ip> 'psql -U mppviewer mppviewer'
```

This is a manual target, not a schedule. Wire it into cron on the box or into a
scheduled job on your machine before you have users worth losing.

Logs
----

```bash
make production/logs/api
make production/logs/parser
ssh mppviewer@<host-ip> 'sudo journalctl -u caddy -f'
```

The server logs `r.URL.Path`, never the query string - share tokens travel in
URLs and must not reach the journal. If a change ever starts logging
`RequestURI`, that is a leak, not a formatting choice.

Local development
-----------------

Production has no Docker, but `docker-compose.yaml` is still the fastest way to
get Postgres and the parser running locally:

```bash
make up      # postgres + migrations + parser in containers
make run     # the Go server on the host, against those
make down
```

`.env` feeds compose only. `.envrc` feeds `make run` via direnv. Neither is read
by anything in production.

Traps
-----

**`PROXIES` must match the number of proxies.** The resolver uses a
rightmost-trusted-count strategy over `X-Forwarded-For`. At `0` behind Caddy,
every request resolves to Caddy's address and all three rate limiters collapse
into one shared bucket for the entire internet. Set higher than the real hop
count, a client can spoof its own IP by sending its own header. One Caddy, one
hop, `PROXIES=1`.

**The parser must bind to loopback.** Spring defaults to `0.0.0.0`. Under Docker
a network namespace hid that; on bare metal it means the sidecar listens on the
public interface, and only ufw stands between it and the internet. The unit sets
`BIND_ADDRESS=127.0.0.1` and `application.yaml` reads it. Do not remove either
half.

**`IPAddressDeny=any` is load-bearing.** The architecture doc requires the
sidecar to have no internet access. That line in
[parser.service](../remote/production/parser.service) is what enforces it - the
process may talk to loopback and nothing else, so a malicious `.mpp` that
reaches code execution has nowhere to call out to. It is not decoration.

**Upload limits live in two repositories.** Go caps free uploads at 10 MB and Pro
at 50 MB (`internal/user/user.go`); the sidecar's `parser.max-body-bytes` is
52428800 - the same 50 MB. Raising the paid tier above that fails at the sidecar
with no useful message. The Caddyfile sets `max_size 64MB` deliberately *above*
both, so an oversized upload gets the application's own JSON 413 rather than a
truncated stream.

**Postgres password and `source`.** `make production/deploy/api` sources
`/etc/mppviewer.env` in a shell to read `$DB_DSN`. The values are single-quoted
so ordinary punctuation is safe, but a password containing a single quote will
break both that and the DSN itself. Generate one without.

**`RESEND_SENDER` on an unverified domain silently fails.**
`onboarding@resend.dev` works for testing only and cannot send to arbitrary
addresses.
