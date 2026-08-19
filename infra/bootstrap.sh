#!/usr/bin/env bash
# EC2 user-data bootstrap for the Gaggle box. Idempotent: safe to re-run when
# Terraform replaces user_data. Terraform interpolates the two ssh keys.
set -euo pipefail

ADMIN_PUBLIC_KEY='${admin_public_key}'
DEPLOY_PUBLIC_KEY='${deploy_public_key}'
DATA_DEV=/dev/nvme1n1
DATA_MOUNT=/data

echo ">> installing docker + compose plugin"
dnf install -y docker docker-compose-plugin
systemctl enable --now docker

echo ">> creating deploy user"
id deploy >/dev/null 2>&1 || useradd -m -s /bin/bash deploy
usermod -aG docker deploy
install -d -o deploy -g deploy -m 700 /home/deploy/.ssh
: > /home/deploy/.ssh/authorized_keys
echo "$ADMIN_PUBLIC_KEY" >> /home/deploy/.ssh/authorized_keys
echo "$DEPLOY_PUBLIC_KEY" >> /home/deploy/.ssh/authorized_keys
chown deploy:deploy /home/deploy/.ssh/authorized_keys
chmod 600 /home/deploy/.ssh/authorized_keys

echo ">> hardening sshd (key-only, no root)"
sed -i 's/^#\?PasswordAuthentication .*/PasswordAuthentication no/' /etc/ssh/sshd_config
sed -i 's/^#\?PermitRootLogin .*/PermitRootLogin no/' /etc/ssh/sshd_config
systemctl try-restart sshd

echo ">> firewall (UFW): 22, 80, 443"
dnf install -y ufw
ufw default deny incoming
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

echo ">> fail2ban"
dnf install -y fail2ban
systemctl enable --now fail2ban

echo ">> mounting data volume at /data"
# The EBS volume attaches after user-data starts (aws_volume_attachment runs
# post-instance). Probe device-node existence — a blank volume is a valid
# target for mkfs, so `blkid` (which fails on blank devices) is the WRONG
# presence probe. Wait for the device node, then format only if unformatted.
echo ">> waiting for data volume $DATA_DEV"
for _ in $(seq 1 30); do
  [ -b "$DATA_DEV" ] && break
  sleep 2
done
if [ ! -b "$DATA_DEV" ]; then
  echo ">> data volume $DATA_DEV never appeared; aborting bootstrap (fix the EBS attach and re-run)" >&2
  exit 1
fi
if ! blkid "$DATA_DEV" >/dev/null 2>&1; then
  mkfs -t xfs "$DATA_DEV"
fi
mkdir -p "$DATA_MOUNT"
grep -q "$DATA_MOUNT" /etc/fstab || echo "$DATA_DEV $DATA_MOUNT xfs defaults,noatime 0 2" >> /etc/fstab
mountpoint -q "$DATA_MOUNT" || mount -a

echo ">> pointing docker data-root at /data/docker"
mkdir -p /data/docker
cat > /etc/docker/daemon.json <<'EOF'
{
  "data-root": "/data/docker"
}
EOF
systemctl restart docker

echo ">> cloning repo for the deploy user"
install -d -o deploy -g deploy /srv
if [[ ! -d /srv/gaggle/.git ]]; then
  # Best-effort: a fresh box has no deploy key yet, so the clone may not be
  # able to authenticate. deploy/apply.sh (via the injected GAGGLE_DEPLOY_KEY)
  # performs its own checkout on the first deploy if this is absent.
  sudo -u deploy git clone git@github.com:ba-reynolds/gaggle.git /srv/gaggle \
    || echo ">> clone deferred to first deploy (deploy key not installed yet)"
else
  echo ">> /srv/gaggle already present; leaving it to deploy/apply.sh"
fi

echo ">> bootstrap complete"