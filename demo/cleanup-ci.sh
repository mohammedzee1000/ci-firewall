#!/usr/bin/env bash
set -euo pipefail

# ==========================================
# Parameter Configuration (Override via ENV)
# ==========================================
NETWORK_NAME="${NETWORK_NAME:-ci-firewall-net}"

# Container Names
JENKINS_CONTAINER_NAME="${JENKINS_CONTAINER_NAME:-jenkins}"
AGENT_1_NAME="${AGENT_1_NAME:-jenkins-agent-1}"
AGENT_2_NAME="${AGENT_2_NAME:-jenkins-agent-2}"
RABBITMQ_CONTAINER_NAME="${RABBITMQ_CONTAINER_NAME:-rabbitmq}"

# Volume Names & Local Paths
JENKINS_VOLUME_NAME="${JENKINS_VOLUME_NAME:-jenkins_home}"
RABBITMQ_VOLUME_NAME="${RABBITMQ_VOLUME_NAME:-rabbitmq_data}"
CONFIG_DIR="./jenkins_config"

echo "==> Stopping and removing containers..."
podman rm -f \
  "${JENKINS_CONTAINER_NAME}" \
  "${AGENT_1_NAME}" \
  "${AGENT_2_NAME}" \
  "${RABBITMQ_CONTAINER_NAME}" 2>/dev/null || true

echo "==> Removing Podman volumes..."
podman volume rm "${JENKINS_VOLUME_NAME}" "${RABBITMQ_VOLUME_NAME}" 2>/dev/null || true

echo "==> Removing network..."
podman network rm "${NETWORK_NAME}" 2>/dev/null || true

echo "==> Deleting local temporary config files..."
rm -rf "${CONFIG_DIR}"

echo "=========================================================="
echo " Cleanup Complete! Environment wiped."
echo "=========================================================="
