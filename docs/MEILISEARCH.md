# Running Meilisearch on the VPS

Standalone binary, no Docker. Meilisearch backs full-text search for
`arkana-frontend`'s posts (see that repo's
`2026-07-11-db-content-migration-sqlite-design.md`, Phase 3).

## Install

```bash
curl -L https://install.meilisearch.com | sh
```

Drops a self-contained `./meilisearch` executable for the host's OS/arch —
no package manager involved.

## Run

```bash
./meilisearch --env production \
  --master-key="<your-key>" \
  --http-addr 127.0.0.1:7700 \
  --db-path /opt/arkana/meili_data
```

- `--env production` — enforces the master key and disables dev conveniences.
  This is a real deployment, not a scratch box.
- `--http-addr 127.0.0.1:7700` — **localhost only**, not `0.0.0.0`. Same
  posture as the Go backend and `blog.db`: no public HTTP surface. Reach it
  from your local machine over an SSH tunnel:
  ```bash
  ssh -L 7700:localhost:7700 <remote-host>
  ```
  then hit `http://localhost:7700` locally.
- `--db-path` — where index data persists to disk. Keep it alongside
  `blog.db` rather than defaulting to the binary's own directory.

## Run as a Service (Recommended)

Running it directly ties it to your SSH session — it dies on disconnect.
For anything beyond a quick test, run it under systemd, same pattern as the
Go API (see `DEPLOYMENT.md`):

First, put the master key in its own file rather than the unit file itself —
unit files under `/etc/systemd/system/` are typically world-readable (`644`),
and anything on the `ExecStart=` line also leaks into `ps aux` /
`systemctl status` output for any local user. Meilisearch reads the key as
an env var directly (`MEILI_MASTER_KEY`), so it doesn't need to be a CLI flag
at all:

```bash
sudo install -m 600 -o root -g root /dev/null /opt/arkana/meilisearch.env
sudo nano /opt/arkana/meilisearch.env
```
```bash
# /opt/arkana/meilisearch.env — mode 600, root-owned, never committed
MEILI_MASTER_KEY=<your-key>
```

Then reference it from the unit via `EnvironmentFile=`, and drop
`--master-key` from `ExecStart=` entirely:

```bash
sudo nano /etc/systemd/system/meilisearch.service
```
```ini
[Unit]
Description=Meilisearch
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/arkana
EnvironmentFile=/opt/arkana/meilisearch.env
ExecStart=/opt/arkana/meilisearch --env production --http-addr 127.0.0.1:7700 --db-path /opt/arkana/meili_data
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```
```bash
sudo systemctl daemon-reload
sudo systemctl enable meilisearch
sudo systemctl start meilisearch
sudo systemctl status meilisearch
```

(`Environment=MEILI_MASTER_KEY=...` directly in `[Service]` was the other
option, but it has the same problem as putting it in `ExecStart=` — the
value sits in the unit file itself, which `systemctl cat`/`show` and any
world-read access exposes. `EnvironmentFile=` only exposes the *path*; the
value stays in a file you control the permissions on.)

## Notes

- The master key lives only in `/opt/arkana/meilisearch.env` (mode 600) —
  never in the unit file, never committed. `arkana-frontend`'s indexing
  scripts (`scripts/indexing/`) read the same value locally via the
  `MEILI_KEY` env var — same secret, same convention, different machine.
- No firewall/reverse-proxy rule should ever forward 7700 publicly — SSH
  tunneling is the only intended access path, matching how `blog.db` is
  reached today.
