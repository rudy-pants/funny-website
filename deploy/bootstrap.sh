#!/usr/bin/env bash
set -Eeuo pipefail

export DEBIAN_FRONTEND=noninteractive
operator_ipv4="50.35.34.51"

apt-get update
apt-get -y upgrade
apt-get install -y ca-certificates curl fail2ban git golang-go ufw unattended-upgrades gnupg debian-keyring debian-archive-keyring apt-transport-https

# A small swapfile keeps the $4/month plan comfortable during package updates.
if [[ ! -e /swapfile ]]; then
  fallocate -l 1G /swapfile
  chmod 600 /swapfile
  mkswap /swapfile
  swapon /swapfile
  echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi

if ! id funnyjokes >/dev/null 2>&1; then
  useradd --system --home-dir /srv/funny-jokes --create-home --shell /usr/sbin/nologin funnyjokes
fi
install -d -o funnyjokes -g funnyjokes -m 0750 /srv/funny-jokes
install -d -o root -g funnyjokes -m 0750 /etc/funny-jokes
install -d -o root -g root -m 0755 /var/log/funny-jokes

# Keep administration on a named, non-root account. The key injected by
# DigitalOcean is copied before root SSH is disabled.
if ! id codex >/dev/null 2>&1; then
  useradd --create-home --shell /bin/bash codex
fi
usermod -aG funnyjokes codex
passwd -l codex >/dev/null 2>&1 || true
install -d -o codex -g codex -m 0700 /home/codex/.ssh
install -o codex -g codex -m 0600 /root/.ssh/authorized_keys /home/codex/.ssh/authorized_keys
chmod -R g+rwX /srv/funny-jokes

# Install Caddy from its official Debian repository.
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
  | gpg --dearmor --yes -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
  -o /etc/apt/sources.list.d/caddy-stable.list
apt-get update
apt-get install -y caddy

# Deny unsolicited traffic at the host layer. Cloud Firewall rules mirror this.
ufw default deny incoming
ufw default allow outgoing
ufw allow from "${operator_ipv4}/32" to any port 22 proto tcp comment 'operator SSH'
ufw allow 80/tcp comment 'public HTTP for ACME'
ufw allow 443/tcp comment 'public HTTPS'
ufw --force enable

cat >/etc/ssh/sshd_config.d/99-funny-jokes-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin no
PubkeyAuthentication yes
X11Forwarding no
AllowAgentForwarding no
AllowUsers codex
EOF
sshd -t
systemctl reload ssh

cat >/etc/sudoers.d/codex-funny-jokes <<'EOF'
codex ALL=(root) NOPASSWD: /usr/bin/systemctl daemon-reload, /usr/bin/systemctl restart funny-jokes.service, /usr/bin/systemctl is-active funny-jokes.service, /usr/bin/systemctl status funny-jokes.service, /usr/bin/systemctl reload caddy, /usr/bin/journalctl -u funny-jokes.service, /usr/bin/journalctl -u caddy
EOF
chmod 0440 /etc/sudoers.d/codex-funny-jokes
visudo -cf /etc/sudoers.d/codex-funny-jokes

cat >/etc/fail2ban/jail.d/funny-jokes-sshd.local <<'EOF'
[sshd]
enabled = true
backend = systemd
maxretry = 5
findtime = 10m
bantime = 1h
EOF
systemctl enable --now fail2ban

systemctl enable --now unattended-upgrades
systemctl enable caddy

echo 'Bootstrap complete.'
