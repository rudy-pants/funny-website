# Provisioning a Secure DigitalOcean Droplet with `doctl`

This is a reusable, operator-run runbook for a small Ubuntu Go web app on a DigitalOcean Droplet. It covers CLI setup, discovery, provisioning, Cloud Firewall and DNS configuration, access, safe changes, cleanup, and troubleshooting.

The examples are intentionally parameterized. Replace every `<PLACEHOLDER>` before running a command. This document does not provision anything by itself.

## Example topology

The commands below use one consistent example:

- Droplet: `funny-web-prod`
- OS: Ubuntu 24.04 LTS (`ubuntu-24-04-x64`)
- App: `/usr/local/bin/funny-web`, running as the unprivileged `funny-web` user on `127.0.0.1:8080`
- Reverse proxy: Nginx on TCP ports 80 and 443
- DNS name: `app.example.com`
- SSH: TCP port 22 from the operator’s current public IPv4 `/32` and IPv6 `/128` only
- IPv6: enabled on the Droplet, with an optional AAAA record

If IPv6 is not required, omit `--enable-ipv6`, the IPv6 firewall rules, the IPv6 SSH rule, and the AAAA record.

## Safety and security rules

- Never commit DigitalOcean API tokens, SSH private keys, cloud-init files containing secrets, or generated credentials. Keep them in a password manager, secret manager, or an untracked file outside the repository.
- The token entered into `doctl auth init` is stored in the local `doctl` configuration. Use a dedicated token for this operator or automation context, choose the narrowest scopes or permissions the control panel offers, keep its lifetime short when possible, and revoke it after one-off provisioning.
- Use the SSH public key in DigitalOcean; keep the matching private key only on the operator’s workstation. `doctl` does not need the private key to create a Droplet.
- Use the operator’s real public egress IP or VPN/bastion CIDR for SSH. `0.0.0.0/0` and `::/0` are not acceptable SSH sources for normal operation.
- Cloud Firewalls and host firewalls are separate. A Cloud Firewall is enforced outside the Droplet; UFW, firewalld, nftables, and iptables run inside the Droplet. Their rules are not synchronized, and traffic must pass both policies.
- Do not put API tokens, private keys, database passwords, or other secrets in user-data. User-data is configuration passed during first boot and may be visible through cloud-init or provider metadata.
- Keep the first SSH session open while testing a new SSH rule, host firewall, or `sshd` configuration from a second session.

## 1. Install and authenticate `doctl`

Install the official CLI using the package manager for the operator’s platform. Examples:

```sh
# macOS
brew install doctl

# Ubuntu/Debian or another Snap-supported Linux workstation
sudo snap install doctl

doctl version
```

For another platform, use the official [doctl installation guide](https://docs.digitalocean.com/reference/doctl/how-to/install/) and verify the downloaded release before placing it on `PATH`. With the Ubuntu Snap package, `doctl compute ssh` may also require:

```sh
sudo snap connect doctl:ssh-keys :ssh-keys
```

Create a named authentication context. `doctl` prompts for the token; do not put the token directly in a shell command or a checked-in file.

```sh
export DOCTL_CONTEXT="<DOCTL_CONTEXT>"

doctl auth init --context "$DOCTL_CONTEXT"
doctl auth list
doctl --context "$DOCTL_CONTEXT" account get
```

For CI, inject a short-lived token from the CI secret store and pass it with the global `--access-token "$DIGITALOCEAN_TOKEN"` option. Do not echo it, print it in debug logs, or save it as an artifact. Never use `doctl auth token` in a log: that command prints the current token.

## 2. Choose the SSH key, region, image, and size

List the SSH keys already registered on the account. Use the ID or fingerprint from this output in the create command.

```sh
doctl --context "$DOCTL_CONTEXT" compute ssh-key list \
  --format ID,Name,FingerPrint
```

If a key must be created, do so locally and upload only its public half. This is optional and is not needed when an existing key is suitable:

```sh
ssh-keygen -t ed25519 -f "$HOME/.ssh/<KEY_FILE_NAME>" -C "<KEY_COMMENT>"
chmod 600 "$HOME/.ssh/<KEY_FILE_NAME>"
doctl --context "$DOCTL_CONTEXT" compute ssh-key import "<KEY_NAME>" \
  --public-key-file "$HOME/.ssh/<KEY_FILE_NAME>.pub"
```

Select a region with `Available=true`:

```sh
doctl --context "$DOCTL_CONTEXT" compute region list \
  --format Slug,Name,Available
```

List public distribution images and choose an Ubuntu slug. Confirm that the image’s minimum disk requirement fits the selected size:

```sh
doctl --context "$DOCTL_CONTEXT" compute image list-distribution --public \
  --format ID,Distribution,Slug,MinDisk
```

List sizes, capacity, and current pricing before choosing a small plan:

```sh
doctl --context "$DOCTL_CONTEXT" compute size list \
  --format Slug,Description,Memory,VCPUs,Disk,PriceMonthly
```

Set the values selected from those commands. The values below are examples, not a claim that they are available in every account or region.

```sh
export DROPLET_NAME="funny-web-prod"
export REGION="<AVAILABLE_REGION_SLUG>"       # example: nyc1
export IMAGE="ubuntu-24-04-x64"
export SIZE="s-1vcpu-1gb"
export SSH_KEY_FINGERPRINT="<SSH_KEY_ID_OR_FINGERPRINT>"
export SSH_PRIVATE_KEY="$HOME/.ssh/<LOCAL_PRIVATE_KEY>"
export DROPLET_TAG="funny-web"
export DOMAIN="example.com"
export HOST="app"                             # creates app.example.com

# Replace these with the operator’s real public egress ranges.
export OPERATOR_IPV4_CIDR="<OPERATOR_PUBLIC_IPV4>/32"
export OPERATOR_IPV6_CIDR="<OPERATOR_PUBLIC_IPV6>/128"
```

## 3. Prepare first-boot user-data

Create a local, untracked file at `<USER_DATA_FILE>` from the cloud-init example below. Substitute `<APP_BINARY_URL>`, `<APP_BINARY_SHA256>`, and `<APP_HOSTNAME>` before using it. The artifact URL must use HTTPS and the checksum must be calculated from the exact release binary.

This example installs Nginx, downloads a pre-built Go binary, verifies it, runs it as a non-root service on `127.0.0.1:8080`, and proxies HTTP traffic on port 80. It leaves certificate/HTTPS setup to the application’s normal TLS workflow; port 443 is opened in the perimeter firewall so it can be enabled later.

```yaml
#cloud-config
package_update: true
packages:
  - ca-certificates
  - curl
  - nginx

write_files:
  - path: /etc/funny-web.env
    owner: root:root
    permissions: '0644'
    content: |
      APP_ADDR=127.0.0.1:8080

  - path: /etc/systemd/system/funny-web.service
    owner: root:root
    permissions: '0644'
    content: |
      [Unit]
      Description=Funny Go web app
      After=network-online.target
      Wants=network-online.target

      [Service]
      User=funny-web
      Group=funny-web
      EnvironmentFile=/etc/funny-web.env
      ExecStart=/usr/local/bin/funny-web
      Restart=on-failure
      RestartSec=5
      NoNewPrivileges=true
      PrivateTmp=true
      ProtectSystem=strict
      ProtectHome=true
      ReadWritePaths=/var/lib/funny-web

      [Install]
      WantedBy=multi-user.target

  - path: /etc/nginx/sites-available/funny-web
    owner: root:root
    permissions: '0644'
    content: |
      server {
          listen 80 default_server;
          listen [::]:80 default_server;
          server_name <APP_HOSTNAME>;

          location / {
              proxy_pass http://127.0.0.1:8080;
              proxy_http_version 1.1;
              proxy_set_header Host $host;
              proxy_set_header X-Real-IP $remote_addr;
              proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
              proxy_set_header X-Forwarded-Proto $scheme;
          }
      }

runcmd:
  - |
    set -eu
    if ! id funny-web >/dev/null 2>&1; then
      useradd --system --home-dir /var/lib/funny-web --create-home \
        --shell /usr/sbin/nologin funny-web
    fi

    curl --fail --show-error --location --proto '=https' --tlsv1.2 \
      --output /usr/local/bin/funny-web '<APP_BINARY_URL>'
    printf '%s  %s\n' '<APP_BINARY_SHA256>' /usr/local/bin/funny-web \
      | sha256sum --check --status
    chown root:root /usr/local/bin/funny-web
    chmod 0755 /usr/local/bin/funny-web

    ln -sfn /etc/nginx/sites-available/funny-web \
      /etc/nginx/sites-enabled/funny-web
    rm -f /etc/nginx/sites-enabled/default
    nginx -t
    systemctl daemon-reload
    systemctl enable --now funny-web.service
    systemctl enable --now nginx
```

The binary must honor `APP_ADDR=127.0.0.1:8080`. If it uses a command-line flag instead, change `ExecStart` accordingly while keeping the Nginx upstream and firewall ports consistent. A failed checksum stops the first-boot script before the binary is started.

Set the local path and protect the file before provisioning:

```sh
export USER_DATA_FILE="<SECURE_LOCAL_PATH>/funny-web-user-data.yaml"
chmod 600 "$USER_DATA_FILE"
```

## 4. Create the Droplet

The SSH key is embedded into the Droplet’s initial `root` account. `--wait` waits for the DigitalOcean create action; it does not wait for cloud-init or the Go service to finish.

```sh
doctl --context "$DOCTL_CONTEXT" compute droplet create "$DROPLET_NAME" \
  --region "$REGION" \
  --image "$IMAGE" \
  --size "$SIZE" \
  --ssh-keys "$SSH_KEY_FINGERPRINT" \
  --tag-name "$DROPLET_TAG" \
  --enable-ipv6 \
  --enable-monitoring \
  --user-data-file "$USER_DATA_FILE" \
  --wait
```

Do not assume the Droplet ID or IP from memory. Retrieve them from the API:

```sh
doctl --context "$DOCTL_CONTEXT" compute droplet get "$DROPLET_NAME" \
  --format ID,Name,Status,PublicIPv4,PublicIPv6,Region,Image,Tags

export DROPLET_ID="<DROPLET_ID_FROM_OUTPUT>"
```

The IPv4 and IPv6 values can be retrieved individually when scripting later steps:

```sh
export DROPLET_IPV4="$(doctl --context "$DOCTL_CONTEXT" \
  compute droplet get "$DROPLET_ID" --format PublicIPv4 --no-header)"
export DROPLET_IPV6="$(doctl --context "$DOCTL_CONTEXT" \
  compute droplet get "$DROPLET_ID" --format PublicIPv6 --no-header)"
```

Confirm that `DROPLET_IPV4` and `DROPLET_IPV6` are real addresses before using them. A Droplet in `new` state may not yet have usable addresses; query again after the status becomes `active`.

## 5. Create and apply a Cloud Firewall

Cloud Firewalls are stateful and allow only traffic covered by an allow rule. The following policy exposes the public web ports, restricts SSH to the operator, and permits the minimum common egress needed for DNS, package updates, artifact download, time synchronization, and monitoring.

The CLI accepts one quoted, space-separated string containing multiple rules. An IPv6 rule uses the `address:::/0` form. If IPv6 is disabled, remove every IPv6 rule below.

```sh
export FIREWALL_NAME="funny-web-prod-fw"

export INBOUND_RULES="protocol:tcp,ports:22,address:${OPERATOR_IPV4_CIDR} \
protocol:tcp,ports:22,address:${OPERATOR_IPV6_CIDR} \
protocol:tcp,ports:80,address:0.0.0.0/0 \
protocol:tcp,ports:80,address:::/0 \
protocol:tcp,ports:443,address:0.0.0.0/0 \
protocol:tcp,ports:443,address:::/0"

export OUTBOUND_RULES="protocol:udp,ports:53,address:0.0.0.0/0 \
protocol:udp,ports:53,address:::/0 \
protocol:tcp,ports:53,address:0.0.0.0/0 \
protocol:tcp,ports:53,address:::/0 \
protocol:tcp,ports:80,address:0.0.0.0/0 \
protocol:tcp,ports:80,address:::/0 \
protocol:tcp,ports:443,address:0.0.0.0/0 \
protocol:tcp,ports:443,address:::/0 \
protocol:udp,ports:123,address:0.0.0.0/0 \
protocol:udp,ports:123,address:::/0"

doctl --context "$DOCTL_CONTEXT" compute firewall create \
  --name "$FIREWALL_NAME" \
  --inbound-rules "$INBOUND_RULES" \
  --outbound-rules "$OUTBOUND_RULES" \
  --droplet-ids "$DROPLET_ID"
```

The create command applies the firewall immediately to the specified Droplet. To use tag-based attachment for a fleet, create the Droplets with a stable tag and use `--tag-names "$DROPLET_TAG"` instead of, or in addition to, individual IDs as appropriate for the account’s resource model.

Retrieve the firewall ID and verify its status, target, rules, and pending changes:

```sh
doctl --context "$DOCTL_CONTEXT" compute firewall list \
  --format ID,Name,Status,DropletIDs,Tags

export FIREWALL_ID="<FIREWALL_ID_FROM_OUTPUT>"

doctl --context "$DOCTL_CONTEXT" compute firewall get "$FIREWALL_ID" \
  --format ID,Name,Status,PendingChanges,DropletIDs,InboundRules,OutboundRules
```

Do not continue until the status is successful and the Droplet is attached. Keep the SSH rule limited to the operator IP; web rules can be public because Nginx is the intended public entry point.

## 6. Connect over SSH and check first boot

`doctl compute ssh` can connect using the Droplet’s name or ID. The standard `ssh` client is also useful for explicit options and a second-session test.

```sh
# Wait for cloud-init without opening an interactive shell.
doctl --context "$DOCTL_CONTEXT" compute ssh "$DROPLET_ID" \
  --ssh-key-path "$SSH_PRIVATE_KEY" \
  --ssh-command 'cloud-init status --wait'

# Interactive initial login; the injected key is initially on root.
ssh -i "$SSH_PRIVATE_KEY" -o IdentitiesOnly=yes \
  "root@${DROPLET_IPV4}"
```

Inside the Droplet, check the app and proxy:

```sh
systemctl --no-pager --full status funny-web.service nginx
curl --fail http://127.0.0.1:8080/
curl --fail -H 'Host: <APP_HOSTNAME>' http://127.0.0.1/
```

For a production host, create a named administrative user, copy the operator’s public key to it, test that account from a second terminal, then disable root and password SSH login. Apply the host firewall only after allowing SSH from the operator CIDR:

```sh
# Run inside the Droplet, after replacing <ADMIN_USER>.
adduser --disabled-password --gecos '' <ADMIN_USER>
usermod --append --groups sudo <ADMIN_USER>
install -d -m 700 -o <ADMIN_USER> -g <ADMIN_USER> \
  /home/<ADMIN_USER>/.ssh
cp /root/.ssh/authorized_keys /home/<ADMIN_USER>/.ssh/authorized_keys
chown <ADMIN_USER>:<ADMIN_USER> /home/<ADMIN_USER>/.ssh/authorized_keys
chmod 600 /home/<ADMIN_USER>/.ssh/authorized_keys

cat >/etc/ssh/sshd_config.d/99-funny-web-hardening.conf <<'EOF'
PermitRootLogin no
PasswordAuthentication no
KbdInteractiveAuthentication no
EOF

sshd -t && systemctl reload ssh
```

Test `ssh -i "$SSH_PRIVATE_KEY" <ADMIN_USER>@<DROPLET_IPV4>` from a second terminal before closing the root session. The `cat` command above is an on-host example; it does not modify this repository.

If using UFW as the host firewall, allow SSH before enabling its default-deny policy:

```sh
apt-get update && apt-get install -y ufw
ufw default deny incoming
ufw default allow outgoing
ufw allow from <OPERATOR_IPV4_CIDR> to any port 22 proto tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable
ufw status verbose
```

Make the host rules compatible with the Cloud Firewall. A Cloud Firewall allow rule does not make a host service listen, and a host firewall cannot grant access that the Cloud Firewall blocks.

## 7. Create DNS A and AAAA records

The domain must be present in DigitalOcean DNS, and the registrar must delegate the zone to the DigitalOcean nameservers. Check the zone before changing it:

```sh
doctl --context "$DOCTL_CONTEXT" compute domain list \
  --format Domain,TTL
doctl --context "$DOCTL_CONTEXT" compute domain records list "$DOMAIN" \
  --format ID,Type,Name,Data,TTL
```

If the zone is not present, add it first and complete nameserver delegation. The `--ip-address` option creates an initial A record; omit it if records will be created explicitly below:

```sh
doctl --context "$DOCTL_CONTEXT" compute domain create "$DOMAIN"
```

Create the A record for `app.example.com` using the retrieved public IPv4. Use a TTL of 300 while changing infrastructure; increase it later if the address is stable.

```sh
doctl --context "$DOCTL_CONTEXT" compute domain records create "$DOMAIN" \
  --record-type A \
  --record-name "$HOST" \
  --record-data "$DROPLET_IPV4" \
  --record-ttl 300
```

Because IPv6 was enabled in the Droplet create command, add the matching AAAA record:

```sh
doctl --context "$DOCTL_CONTEXT" compute domain records create "$DOMAIN" \
  --record-type AAAA \
  --record-name "$HOST" \
  --record-data "$DROPLET_IPV6" \
  --record-ttl 300
```

For the zone apex, use `--record-name "$DOMAIN"` instead of `--record-name "$HOST"`, and verify that no existing A or AAAA record for the same name should be preserved. Record operations are not automatically idempotent: list first to avoid creating duplicates.

Verify both the provider-side records and public resolution:

```sh
doctl --context "$DOCTL_CONTEXT" compute domain records list "$DOMAIN" \
  --format ID,Type,Name,Data,TTL
dig +short A "${HOST}.${DOMAIN}"
dig +short AAAA "${HOST}.${DOMAIN}"
```

DNS caches honor the TTL, so a correct change may not be visible immediately from every resolver.

## 8. Update the Cloud Firewall safely

Treat `firewall update` as a full replacement, not a patch. The current CLI documentation states that omitted attributes reset to defaults. Always inspect the current firewall, reconstruct the complete intended policy, and include the full target set of Droplet IDs or tags.

```sh
# Review before changing anything.
doctl --context "$DOCTL_CONTEXT" compute firewall get "$FIREWALL_ID"

# These variables must contain the complete intended policy, not only a new rule.
doctl --context "$DOCTL_CONTEXT" compute firewall update "$FIREWALL_ID" \
  --name "$FIREWALL_NAME" \
  --inbound-rules "$INBOUND_RULES" \
  --outbound-rules "$OUTBOUND_RULES" \
  --droplet-ids "$DROPLET_ID"

doctl --context "$DOCTL_CONTEXT" compute firewall get "$FIREWALL_ID" \
  --format ID,Name,Status,PendingChanges,DropletIDs,InboundRules,OutboundRules
```

For an additive change, prefer `add-rules`, which preserves existing rules. For example, add HTTPS from a temporary test network, test it, and then remove it using the exact same rule representation:

```sh
doctl --context "$DOCTL_CONTEXT" compute firewall add-rules "$FIREWALL_ID" \
  --inbound-rules "protocol:tcp,ports:443,address:<TEMP_TEST_CIDR>"

# Test from the temporary network, then remove the temporary allow rule.
doctl --context "$DOCTL_CONTEXT" compute firewall remove-rules "$FIREWALL_ID" \
  --inbound-rules "protocol:tcp,ports:443,address:<TEMP_TEST_CIDR>"
```

When an operator’s IP changes, add the new SSH CIDR first, test a second session, and only then remove the old CIDR:

```sh
doctl --context "$DOCTL_CONTEXT" compute firewall add-rules "$FIREWALL_ID" \
  --inbound-rules "protocol:tcp,ports:22,address:<NEW_OPERATOR_IPV4>/32"

# After a successful second SSH login:
doctl --context "$DOCTL_CONTEXT" compute firewall remove-rules "$FIREWALL_ID" \
  --inbound-rules "protocol:tcp,ports:22,address:<OLD_OPERATOR_IPV4>/32"
```

Use `add-droplets` and `remove-droplets` to change firewall membership without rewriting rules:

```sh
doctl --context "$DOCTL_CONTEXT" compute firewall add-droplets "$FIREWALL_ID" \
  --droplet-ids "<ADDITIONAL_DROPLET_ID>"
doctl --context "$DOCTL_CONTEXT" compute firewall remove-droplets "$FIREWALL_ID" \
  --droplet-ids "<DROPLET_ID_TO_DETACH>"
```

After every change, check `Status` and `PendingChanges`, then test from the relevant source network. `doctl` manages allow rules; it does not support the firewall rule `action` field for explicit deny rules. Use the DigitalOcean API if an explicit deny rule is required.

## 9. Cleanup

Cleanup is irreversible for a Droplet and should be performed only after verifying the exact IDs and record IDs. Remove DNS records first so clients stop being directed to a retired address, then destroy the Droplet and its dedicated Cloud Firewall.

```sh
# Inspect records and delete only the A/AAAA records created for this app.
doctl --context "$DOCTL_CONTEXT" compute domain records list "$DOMAIN" \
  --format ID,Type,Name,Data,TTL
doctl --context "$DOCTL_CONTEXT" compute domain records delete "$DOMAIN" \
  <A_RECORD_ID>
doctl --context "$DOCTL_CONTEXT" compute domain records delete "$DOMAIN" \
  <AAAA_RECORD_ID>

# Confirm that the target is the intended Droplet.
doctl --context "$DOCTL_CONTEXT" compute droplet get "$DROPLET_ID" \
  --format ID,Name,Status,PublicIPv4,PublicIPv6,Tags

# Prompts for confirmation; do not use --force until the ID has been checked.
doctl --context "$DOCTL_CONTEXT" compute droplet delete "$DROPLET_ID"

# Confirm no other Droplets use this firewall before deleting it.
doctl --context "$DOCTL_CONTEXT" compute firewall get "$FIREWALL_ID" \
  --format ID,Name,Status,DropletIDs,Tags
doctl --context "$DOCTL_CONTEXT" compute firewall delete "$FIREWALL_ID"
```

If the domain or firewall is shared, do not delete the whole resource. Delete only the app’s records or detach only the app’s Droplet. If the SSH key was created solely for this Droplet and is no longer needed, remove its DigitalOcean key record separately; do not delete a key used by other hosts.

Remove the local authentication context after a one-off task, and revoke the corresponding token in the DigitalOcean control panel:

```sh
doctl auth remove --context "$DOCTL_CONTEXT"
```

Deleting the Droplet does not delete DNS records, Cloud Firewalls, registered SSH keys, reserved IPs, volumes, snapshots, or other attached resources. Check each resource class before considering cleanup complete.

## Troubleshooting

### Authentication or authorization errors

- Run `doctl auth list` and use the intended context explicitly with `--context "$DOCTL_CONTEXT"`.
- Run `doctl --context "$DOCTL_CONTEXT" account get` to distinguish a bad context/token from a resource problem.
- Check token expiry, scope, account/team membership, project access, billing status, and Droplet quota. Create a new short-lived token rather than exposing the old one in a command or log.
- Do not run `doctl auth token` in shared output; it prints the secret.

### Invalid region, image, or size

- Confirm the region has `Available=true`.
- Confirm the image slug exists in `compute image list-distribution` and that the selected size meets `MinDisk`.
- Confirm that the plan is available in the selected region and that the account has capacity, quota, and billing enabled.

### Droplet has no usable IP or is not ready

- Query `compute droplet get` until the status is `active`.
- Confirm `--enable-ipv6` was used and that the `PublicIPv6` field is populated before creating an AAAA record.
- `--wait` covers the provider action, not cloud-init. Check cloud-init separately.

### SSH times out or is refused

- Confirm the current operator public IP has not changed and that the Cloud Firewall contains TCP 22 from `<OPERATOR_PUBLIC_IPV4>/32` and, if applicable, `<OPERATOR_PUBLIC_IPV6>/128`.
- Check the firewall’s `Status`, `PendingChanges`, `DropletIDs`, and `InboundRules`.
- Confirm the private key matches the registered public key, has restrictive permissions, and is passed with `-o IdentitiesOnly=yes` or `--ssh-key-path`.
- Check whether the host firewall or `sshd` hardening was applied before the new administrative login was tested. Keep any working session open during repairs.
- Test the address family explicitly: use the public IPv4 first, then `ssh -6` with the IPv6 address if IPv6 is intended.

### User-data or the service did not run

From an SSH session, inspect:

```sh
cloud-init status --long
sudo sed -n '1,240p' /var/log/cloud-init-output.log
sudo journalctl -u funny-web.service -n 100 --no-pager
sudo systemctl status funny-web.service nginx
sudo nginx -t
```

Common causes are an unreplaced placeholder, an unreachable artifact URL, a checksum mismatch, a binary built for the wrong architecture, a binary that does not honor `APP_ADDR`, or a syntax error in the Nginx configuration.

### Nginx returns 502 or the app is unreachable

- Test `curl http://127.0.0.1:8080/` on the Droplet.
- Confirm `funny-web.service` is active and that the process listens on `127.0.0.1:8080`.
- Confirm Nginx proxies to the same address and that the public request uses the expected `Host` header.
- Cloud Firewall port 80/443 rules do not make an app listen, and they do not replace Nginx or a host firewall.

### DNS does not resolve

- Confirm the domain is in DigitalOcean DNS and the registrar delegates the zone to the DigitalOcean nameservers.
- Confirm the A/AAAA record name, data, and record IDs with `compute domain records list`.
- Query both record types with `dig`; remember that resolvers may cache old values until the prior TTL expires.
- Do not leave an old A or AAAA record pointing at another host unless that is intentional.

### Package updates or first-boot downloads fail after applying the firewall

- Verify outbound DNS on UDP/TCP 53 and HTTPS on TCP 443. Add HTTP on TCP 80 only when a required package or artifact mirror needs it.
- If monitoring is enabled, keep outbound HTTPS available for the monitoring agent.
- Check the host firewall separately; a Cloud Firewall allow rule cannot override UFW or nftables.

## Official references

- [Install and configure `doctl`](https://docs.digitalocean.com/reference/doctl/how-to/install/)
- [`doctl compute droplet create`](https://docs.digitalocean.com/reference/doctl/reference/compute/droplet/create/)
- [`doctl compute firewall create`](https://docs.digitalocean.com/reference/doctl/reference/compute/firewall/create/)
- [`doctl compute firewall update`](https://docs.digitalocean.com/reference/doctl/reference/compute/firewall/update/)
- [Configure Cloud Firewall rules](https://docs.digitalocean.com/products/networking/firewalls/how-to/configure-rules/)
- [Manage DNS records](https://docs.digitalocean.com/products/networking/dns/how-to/manage-records/)
- [`doctl compute ssh`](https://docs.digitalocean.com/reference/doctl/reference/compute/ssh/)
