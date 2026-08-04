# Running Go API on VPS

## Cross-Compile Binary
```bash
# Build for Linux from your local machine
GOOS=linux GOARCH=amd64 go build -o arkana-api main.go
```

## Upload and Run
```bash
# Upload to VPS
scp arkana-api root@YOUR_VULTR_IP:/home/api/

# SSH and make executable
ssh root@YOUR_VULTR_IP
chmod +x /home/api/arkana-api

# Test run
./arkana-api
```

## Run as Service (Recommended)
```bash
sudo nano /etc/systemd/system/arkana-api.service
```
```ini
[Unit]
Description=Arkana API
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/home/api
ExecStart=/home/api/arkana-api
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```
```bash
sudo systemctl daemon-reload
sudo systemctl enable arkana-api
sudo systemctl start arkana-api
sudo systemctl status arkana-api
```

## Redis (required)

The API now calls `rediscache.NewClient` at startup and `log.Fatal`s if
Redis is unreachable - before migrations run, before the router starts. A
missing Redis crash-loops the entire API, not just the quiz endpoints.

**This must be done BEFORE merging/deploying this change** - this repo
auto-deploys on push to `main`, so merging without Redis already
provisioned takes the whole API down.

- Install/run a Redis instance reachable from the VPS - e.g.
  `apt install redis-server`, or a Docker container. Bind it to
  localhost only; no auth needed for a localhost-only bind.
- Add `Environment=REDIS_ADDR=127.0.0.1:6379` (or an `EnvironmentFile=`
  pointing at a file containing that line) to the `[Service]` block of
  the systemd unit above.
- `config.Load`'s default (`localhost:3334`) matches this repo's local
  dev `docker-compose.yml` port mapping, for local convenience only -
  production must set `REDIS_ADDR` explicitly to the real Redis port
  (typically 6379), not rely on that default.

## Notes

- SQLite is embedded in the Go binary (no separate installation needed)
- Ensure `.db` file is in `/home/api` or adjust WorkingDirectory
- nginx already configured to proxy `/api/*` to your backend