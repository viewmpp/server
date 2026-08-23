Deploy
======

One Ubuntu VPS running Docker Compose. Caddy terminates TLS and is the only
container that publishes ports. Deploys are `git pull` plus `docker compose up`
on the host - no registry, no CI, no artifacts shipped from your machine.

Shape
-----

```
internet ──► caddy :80/:443 ──► server:4000 ──► parser:8080
             (published)        (internal)      (internal)
                                     │
                                     └──► server-db:5432 (internal)
```

Only Caddy is reachable from outside. Everything else lives on the compose
network and has no `ports:` entry at all, so there is nothing to reach even
from the host's own loopback.

Files
-----

```
docker-compose.yaml         base: postgres, migrate, parser, server
docker-compose.dev.yaml     override: loopback ports for host development
docker-compose.prod.yaml    override: caddy
remote/production/Caddyfile mounted read-only into the caddy container
remote/setup/01.sh          one-time provisioning of a fresh server
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

**1. Provision.** Copy the script up as root and run it:

```bash
rsync -P ./remote/setup/01.sh root@<host-ip>:~
ssh -t root@<host-ip> 'bash 01.sh'
```

It installs Docker and the Compose plugin, creates the `deploy` user in the
`docker` group, and clones both repositories to `~/mpp-viewer/server` and
`~/mpp-viewer/parser`. The parser build context is `../parser`, so that layout
is required, not cosmetic.

**2. Set a password for the deploy user.** The script deletes it and forces a
change on first login:

```bash
ssh deploy@<host-ip>
```

**3. Write the environment file.** It lives only on the host and is never
committed:

```bash
cd ~/mpp-viewer/server
cp .env.example .env
nano .env
```

Fill in `RESEND_API_KEY` and replace `POSTGRES_PASSWORD`. The rest of the
template is already production-shaped. URL-encode the password if it contains
`@ : / ? # &` - compose interpolates it into the DSN as a string.

**4. Bring it up.**

```bash
docker compose -f docker-compose.yaml -f docker-compose.prod.yaml up -d --build
```

**5. Verify.**

```bash
curl -fsS https://viewmpp.com/api/v1/healthcheck
# {"status":"OK","env":"prod","version":"<sha>"}
```

Deploying an update
-------------------

```bash
ssh deploy@<host-ip>
cd ~/mpp-viewer/server && git pull
cd ../parser && git pull
cd ../server
docker compose -f docker-compose.yaml -f docker-compose.prod.yaml up -d --build
```

Compose rebuilds and recreates only what changed. Migrations run automatically:
the `migrate` service is gated on a healthy database, and `server` waits for it
to exit 0.

Deploys are not zero-downtime. `stop_grace_period: 40s` gives the old container
room to drain its 30-second shutdown context and finish any in-flight
verification email before Docker escalates to SIGKILL.

Local development
-----------------

```bash
docker compose -f docker-compose.yaml -f docker-compose.dev.yaml up -d
```

That publishes Postgres on `127.0.0.1:5432`, the parser on `127.0.0.1:8080` and
the server on `127.0.0.1:4000`.

To iterate on Go without rebuilding an image, start only the backing services
and run the binary on the host - `PARSER_URL` already defaults to
`http://localhost:8080/parse`:

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
| `EARLY_ACCESS_SEATS` | `100` | `0` closes early access |

`PORT`, `DB_DSN` and `PARSER_URL` are set in `docker-compose.yaml` and override
anything in `.env` - they point at compose service names and must not be edited
per-host.

After changing `.env`:

```bash
docker compose -f docker-compose.yaml -f docker-compose.prod.yaml up -d
```

Migrations
----------

Run on every `up`. To run one by hand:

```bash
docker compose run --rm migrate \
  -path /migrations \
  -database "postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@server-db:5432/$POSTGRES_DB?sslmode=disable" \
  down 1
```

If a migration fails halfway, `migrate` marks the schema dirty and refuses to
run again until the version is forced with `force <version>`.

Rollback
--------

```bash
cd ~/mpp-viewer/server
git checkout <previous-sha>
docker compose -f docker-compose.yaml -f docker-compose.prod.yaml up -d --build
```

If the rolled-back code predates a migration that already ran, roll the
migration back **first**. `migrate` runs before the server starts, so a
forward-only schema paired with backward code fails at query time, not at boot,
which is the harder failure to read.

Backups
-------

The database is the only state that matters. A stored contract cannot be
regenerated - the uploaded `.mpp` is deleted immediately after parsing, so for a
signed-in user the row in Postgres is the only remaining copy.

```bash
docker compose exec -T server-db \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" | gzip > backup-$(date +%F).sql.gz
```

Restore:

```bash
gunzip -c backup-2026-01-01.sql.gz | \
  docker compose exec -T server-db psql -U "$POSTGRES_USER" "$POSTGRES_DB"
```

This is manual. Put it in cron on the host before you have users worth losing.

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

**`RESEND_SENDER` on an unverified domain silently fails.**
`onboarding@resend.dev` works for testing only and cannot send to arbitrary
addresses.
