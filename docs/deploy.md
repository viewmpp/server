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
# {"status":"OK","env":"prod","version":"<sha>"}
```

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
anyone who can reach it. Today that is `cmdline` and `memstats`, neither of them
sensitive - secrets arrive through the environment, not as flags. Adding
counters carrying business data would change that judgement.

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
| `SECRET_KEY` | 32+ random characters | Signs every csrf token. The process refuses to start in prod without it |
| `EARLY_ACCESS_SEATS` | `100` | `0` closes early access |
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

Backups
-------

The database is the only state that matters. A stored contract cannot be
regenerated - the uploaded `.mpp` is deleted immediately after parsing, so for a
signed-in user the row in Postgres is the only remaining copy.

Nothing is being backed up right now. `remote/production/mpp-backup.service` and
`mpp-backup.timer` are in the repository but have never been installed, and they
would fail if they were: they point at `/home/dzenthai/mpp-viewer` while `init.sh`
creates `/viewmpp`, and their `ExecStart` names `remote/production/backup.sh`,
which does not exist. The script has to be written and the paths fixed before the
timer means anything.

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
