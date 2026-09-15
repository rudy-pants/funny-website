# Funny Jokes deployment

This directory contains a systemd unit and Caddy reverse-proxy template for
running the Go application on Ubuntu. The examples assume:

- Ubuntu 22.04 or 24.04 with systemd.
- The application binary is `/srv/funny-jokes/funny-jokes`.
- The application supports `ADDR` and listens on `127.0.0.1:8080`.
- The public DNS name is substituted for `DOMAIN` in `Caddyfile`.
- Caddy is installed from its official Ubuntu package/repository.
- The bootstrap creates a non-root `codex` SSH account and disables root SSH.

If the application uses a different configuration variable or command-line
flag for its listen address, adjust the environment-file example and verify
the resulting socket is still localhost-only before exposing the service.

## Deploy

Run these commands from the repository checkout, substituting the real binary
source and domain where indicated. The checked-in `deploy/bootstrap.sh` also
performs the initial host setup and creates the restricted `codex` administrator
account from the injected DigitalOcean SSH key.

### 1. Install host packages and create the service account

```sh
sudo apt update
sudo apt install -y caddy ufw unattended-upgrades

getent group funnyjokes >/dev/null || sudo groupadd --system funnyjokes
id funnyjokes >/dev/null 2>&1 || \
  sudo useradd --system --gid funnyjokes --home-dir /srv/funny-jokes \
  --shell /usr/sbin/nologin funnyjokes

sudo install -d -o funnyjokes -g funnyjokes -m 0755 /srv/funny-jokes
sudo install -d -o root -g funnyjokes -m 0750 /etc/funny-jokes
```

If `funnyjokes` already exists, confirm that it is a locked-down service
account and belongs to the `funnyjokes` group before continuing.

### 2. Install the binary

Build the Go application using the project’s normal release process, then
install its output with ownership and permissions like this:

```sh
sudo install -o funnyjokes -g funnyjokes -m 0755 /path/to/funny-jokes \
  /srv/funny-jokes/funny-jokes
```

The service does not need write access to the application directory. If the
application must persist files, grant write access only to a dedicated data
directory and add that directory to the unit with `ReadWritePaths=`.

### 3. Create the private environment file

Create the file outside the repository. Keep it readable by root and the
service group only:

```sh
sudo touch /etc/funny-jokes/funny-jokes.env
sudo chown root:funnyjokes /etc/funny-jokes/funny-jokes.env
sudo chmod 0640 /etc/funny-jokes/funny-jokes.env
sudoedit /etc/funny-jokes/funny-jokes.env
```

At minimum, configure the listen address:

```dotenv
ADDR=127.0.0.1:8080
```

Add any application secrets or other settings there. Do not commit this file
or copy its contents into a tracked file.

### 4. Install and start systemd service

```sh
sudo install -o root -g root -m 0644 deploy/funny-jokes.service \
  /etc/systemd/system/funny-jokes.service
sudo systemd-analyze verify /etc/systemd/system/funny-jokes.service
sudo systemctl daemon-reload
sudo systemctl enable --now funny-jokes.service
sudo systemctl status funny-jokes.service
```

The service is deliberately configured without a public socket. Its restart
policy allows five starts in 60 seconds before systemd rate-limits failures.

### 5. Configure DNS, firewall, and Caddy

Create `A` and, if used, `AAAA` DNS records for the real domain pointing to
the server. Replace `DOMAIN` in a copy of `Caddyfile`, then validate and load
it:

```sh
sudo install -o root -g root -m 0644 deploy/Caddyfile /etc/caddy/Caddyfile
sudoedit /etc/caddy/Caddyfile
sudo caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
sudo systemctl enable --now caddy
sudo systemctl reload caddy
```

Allow SSH only from a trusted administrator network. Replace
`ADMIN_CIDR` with that network; do not use `0.0.0.0/0` unless there is no
safer administrative path.

```sh
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow from ADMIN_CIDR to any port 22 proto tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
sudo ufw status verbose
```

Ports 80 and 443 must be public for normal HTTP access and Caddy’s automatic
certificate issuance. Do not open port 8080; it should remain bound only to
localhost.

## Security checklist

- Use SSH keys, disable password authentication and direct root SSH login, and
  restrict port 22 to a trusted CIDR, VPN, or bastion.
- Expose only TCP 80 and 443 publicly; keep TCP 8080 private and localhost-only.
- Enable and verify Ubuntu unattended security upgrades:

  ```sh
  sudo systemctl enable --now unattended-upgrades
  sudo systemctl status unattended-upgrades
  ```

- Review service and proxy logs, and configure suitable journald retention and
  host monitoring:

  ```sh
  sudo journalctl -u funny-jokes.service --since today
  sudo journalctl -u caddy --since today
  ```

- Keep secrets only in `/etc/funny-jokes/funny-jokes.env` or another approved
  secret store; never place them in the repository, binary build arguments, or
  Caddyfile.
- Review the unit’s hardening after installation:

  ```sh
  systemd-analyze security funny-jokes.service
  ```

## Test and troubleshoot

Run the checks below after every deployment. Substitute the actual domain for
`DOMAIN` in the commands that use it.

```sh
sudo systemctl is-active --quiet funny-jokes.service
sudo systemctl is-active --quiet caddy
sudo ss -ltnp | grep ':8080'
curl --fail http://127.0.0.1:8080/
curl --fail --resolve DOMAIN:80:127.0.0.1 http://DOMAIN/
curl --fail https://DOMAIN/
```

The socket check should show `127.0.0.1:8080` (or equivalent localhost
binding), never `0.0.0.0:8080` or `[::]:8080`. The final HTTPS check should be
run using the real public DNS name so certificate validation is meaningful.
For failures, inspect `journalctl -u funny-jokes.service` and
`journalctl -u caddy`, then re-run the systemd and Caddy validation commands.
