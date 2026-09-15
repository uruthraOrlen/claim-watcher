# Claim Watcher

Standalone one-shot Go worker for the Claims Management system.

## Behaviour

Each run:

1. Connects to `claim_db`.
2. Counts all non-terminal claims.
3. Selects every non-terminal claim that is:
   - overdue, or
   - due today, or
   - due within `WARNING_DAYS` (default 14 days).
4. Excludes terminal states:
   - `ABGESCHLOSSEN`
   - `ABGELEHNT`
5. Sends one consolidated email to `SMTP_TO`.
6. Records one `claim_notification` audit row per included claim.
7. Updates `claim_watcher_run`.
8. Exits.

If no claims match, no email is sent and the run is recorded as `NO_ACTION`.

## Notification classification

- `OVERDUE`: deadline before today.
- `DUE_TODAY`: deadline is today.
- `UPCOMING`: deadline is between tomorrow and `WARNING_DAYS`.

Display severity:
- overdue: `OVERDUE`
- due today: `DUE_TODAY`
- 1–7 days: `CRITICAL`
- 8–14 days: `URGENT`

## Requirements

- Go 1.25+
- PostgreSQL access to `claim_db`
- Existing tables:
  - `claim_input`
  - `app_user`
  - `claim_notification`
  - `claim_watcher_run`
- SMTP relay access

## Local setup

Copy the environment template:

```bash
cp .env.example .env
```

Load the environment variables before running. For example:

```bash
set -a
source .env
set +a
```

Install dependencies:

```bash
go mod tidy
```

Run:

```bash
go run ./cmd/claim-watcher
```

Build:

```bash
go build -o claim-watcher ./cmd/claim-watcher
```

## Docker

Build:

```bash
docker build -t claim-watcher:1.0.0 .
```

Run once:

```bash
docker run --rm \
  --env-file .env \
  --add-host=postgres-host:172.18.0.1 \
  claim-watcher:1.0.0
```

If the container is attached to a Docker network that already resolves `postgres-host`, omit `--add-host`.

## Important deployment model

This application intentionally does **not** contain an internal scheduler.

It is a one-shot worker: run, notify, audit, exit.

Schedule the container once per day using cron or a systemd timer. That avoids keeping an otherwise idle process alive and makes execution history easy to inspect.

## SMTP TLS modes

`SMTP_TLS_MODE` supports:

- `none` — plaintext SMTP; intended only for a trusted internal relay.
- `opportunistic` — use STARTTLS if the server supports it.
- `starttls` — require STARTTLS.

If the relay requires authentication, set both `SMTP_USERNAME` and `SMTP_PASSWORD`.

## Claim Manager links

If `CLAIMS_BASE_URL` is set, claim numbers in the HTML digest link to:

```text
<CLAIMS_BASE_URL>/claims/<claim UUID>
```

If the Claim Manager uses a different route, adjust `claimLink()` in:

```text
internal/watcher/digest.go
```

## Audit semantics

Each successful digest inserts a `claim_notification` row for every claim included in that day's digest.

`event_group_id` is set to the `claim_watcher_run.id`, so all notification rows belonging to one email/run can be queried together.

No notification rows are inserted if SMTP delivery fails.
