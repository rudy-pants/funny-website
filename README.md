# Joke Atlas

Joke Atlas is a tiny Go + HTMX exploration game. Wander a hand-drawn 2D map,
unlock joke regions, and collect family-friendly field notes. It is stateless,
has no sign-up or tracking, and works with or without JavaScript.

## Run locally

```sh
go test ./...
go run ./cmd/server
```

Open <http://127.0.0.1:8080>. Set `ADDR` to change the listening address.

## Deploy

The live low-cost deployment is currently available at
<https://147.182.229.57.sslip.io>.

- `deploy/bootstrap.sh` installs Go, Caddy, UFW, fail2ban, unattended security
  upgrades, swap, and a locked-down non-root `codex` administrator.
- `deploy/funny-jokes.service` runs the app as the unprivileged `funnyjokes`
  user on `127.0.0.1:8080`.
- `deploy/Caddyfile` terminates HTTPS and proxies only to the local app.
- `deploy/README.md` explains the deployment and security checks.
- `docs/DIGITALOCEAN_CLI.md` is a reusable `doctl` provisioning runbook for
  another agent or GitHub operator.

The current hostname is an IP-derived `sslip.io` name because this DigitalOcean
account has no custom domain configured. To use a permanent domain, create an A
record pointing to the Droplet’s public IP, replace `DOMAIN` in the Caddyfile,
and reload Caddy.

## Security posture

The Droplet uses SSH keys, disables password and root SSH login, restricts SSH
to the operator’s current public IP, exposes only ports 80 and 443, keeps the
Go service private to localhost, applies both Cloud Firewall and UFW rules,
uses a hardened systemd unit, and serves a restrictive browser security policy.
Keep the operator IP in `deploy/bootstrap.sh` and the Cloud Firewall current if
your network changes.
