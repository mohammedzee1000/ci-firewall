#!/usr/bin/env bash
set -euo pipefail

# ==========================================
# Parameter Configuration (Override via ENV)
# ==========================================
NETWORK_NAME="${NETWORK_NAME:-ci-firewall-net}"

# RabbitMQ Settings
RABBITMQ_CONTAINER_NAME="${RABBITMQ_CONTAINER_NAME:-rabbitmq}"
RABBITMQ_IMAGE="${RABBITMQ_IMAGE:-docker.io/library/rabbitmq:3-management}"
RABBITMQ_USER="${RABBITMQ_USER:-guest}"
RABBITMQ_PASS="${RABBITMQ_PASS:-guest}"
RABBITMQ_AMQP_PORT="${RABBITMQ_AMQP_PORT:-5672}"
RABBITMQ_MGMT_PORT="${RABBITMQ_MGMT_PORT:-15672}"
RABBITMQ_VOLUME_NAME="${RABBITMQ_VOLUME_NAME:-rabbitmq_data}"

# Jenkins Controller Settings
JENKINS_CONTAINER_NAME="${JENKINS_CONTAINER_NAME:-jenkins}"
JENKINS_IMAGE="${JENKINS_IMAGE:-docker.io/jenkins/jenkins:lts-jdk17}"
JENKINS_HTTP_PORT="${JENKINS_HTTP_PORT:-8080}"
JENKINS_AGENT_PORT="${JENKINS_AGENT_PORT:-50000}"
JENKINS_ADMIN_USER="${JENKINS_ADMIN_USER:-jenkins}"
JENKINS_ADMIN_PASS="${JENKINS_ADMIN_PASS:-jenkins}"
JENKINS_VOLUME_NAME="${JENKINS_VOLUME_NAME:-jenkins_home}"

# Jenkins Agent Settings
AGENT_IMAGE="${AGENT_IMAGE:-docker.io/jenkins/inbound-agent:latest-jdk17}"
AGENT_1_NAME="${AGENT_1_NAME:-jenkins-agent-1}"
AGENT_2_NAME="${AGENT_2_NAME:-jenkins-agent-2}"

# Local Config Setup
CONFIG_DIR="$(pwd)/jenkins_config"
JCasC_FILE="${CONFIG_DIR}/jenkins.yaml"
INIT_GROOVY_FILE="${CONFIG_DIR}/01-agents.groovy"
RABBITMQ_PLUGINS_FILE="${CONFIG_DIR}/enabled_plugins"

# ==========================================
# 1. Network & Directory Setup
# ==========================================
echo "==> Setting up Podman network..."
podman network create "${NETWORK_NAME}" 2>/dev/null || true
mkdir -p "${CONFIG_DIR}"

# ==========================================
# 2. Deploy RabbitMQ with Mounted Enabled Plugins
# ==========================================
echo "==> Generating RabbitMQ enabled_plugins file..."
cat <<EOF > "${RABBITMQ_PLUGINS_FILE}"
[rabbitmq_management,rabbitmq_jms_topic_exchange,rabbitmq_amqp1_0].
EOF

echo "==> Launching RabbitMQ container..."
podman run -d \
  --name "${RABBITMQ_CONTAINER_NAME}" \
  --network "${NETWORK_NAME}" \
  --dns 8.8.8.8 --dns 1.1.1.1 \
  --restart unless-stopped \
  -p "${RABBITMQ_AMQP_PORT}:5672" \
  -p "${RABBITMQ_MGMT_PORT}:15672" \
  -e RABBITMQ_DEFAULT_USER="${RABBITMQ_USER}" \
  -e RABBITMQ_DEFAULT_PASS="${RABBITMQ_PASS}" \
  -v "${RABBITMQ_PLUGINS_FILE}:/etc/rabbitmq/enabled_plugins:Z" \
  -v "${RABBITMQ_VOLUME_NAME}:/var/lib/rabbitmq:Z" \
  "${RABBITMQ_IMAGE}"

# ==========================================
# 3. Generate JCasC Configuration (With Full Admin Authorization)
# ==========================================
echo "==> Generating JCasC configuration..."
cat <<EOF > "${JCasC_FILE}"
jenkins:
  securityRealm:
    local:
      allowsSignup: false
      users:
        - id: "${JENKINS_ADMIN_USER}"
          password: "${JENKINS_ADMIN_PASS}"
  authorizationStrategy:
    loggedInUsersCanDoAnything:
      allowAnonymousRead: false
  agentProtocols:
    - "JNLP4-connect"
  slaveAgentPort: ${JENKINS_AGENT_PORT}

unclassified:
  location:
    url: "http://localhost:${JENKINS_HTTP_PORT}/"
EOF

# ==========================================
# 4. Generate Init Groovy Script for Node Provisioning
# ==========================================
echo "==> Generating Init Groovy Script..."
cat <<EOF > "${INIT_GROOVY_FILE}"
import hudson.model.*
import hudson.slaves.*
import jenkins.model.*

def getOrCreateAgentSecret(String name) {
    def instance = Jenkins.get()
    def node = instance.getNode(name)
    if (node == null) {
        node = new DumbSlave(
            name,
            "/home/jenkins/agent",
            new JNLPLauncher(true)
        )
        node.setNumExecutors(2)
        node.setMode(Node.Mode.NORMAL)
        node.setRetentionStrategy(RetentionStrategy.Always.INSTANCE)
        instance.addNode(node)
    }
    return node.getComputer().getJnlpMac()
}

def secret1 = getOrCreateAgentSecret("${AGENT_1_NAME}")
def secret2 = getOrCreateAgentSecret("${AGENT_2_NAME}")

new File("/var/jenkins_home/secrets.txt").text = "SECRET_1=\${secret1}\nSECRET_2=\${secret2}\n"
EOF

# ==========================================
# 5. Deploy Jenkins Controller
# ==========================================
echo "==> Launching Jenkins Controller..."
podman run -d \
  --name "${JENKINS_CONTAINER_NAME}" \
  --network "${NETWORK_NAME}" \
  --dns 8.8.8.8 --dns 1.1.1.1 \
  --restart unless-stopped \
  -p "${JENKINS_HTTP_PORT}:8080" \
  -p "${JENKINS_AGENT_PORT}:${JENKINS_AGENT_PORT}" \
  -e JAVA_OPTS="-Djenkins.install.runSetupWizard=false -Djenkins.model.Jenkins.slaveAgentPort=${JENKINS_AGENT_PORT}" \
  -e CASC_JENKINS_CONFIG="/var/jenkins_home/jenkins.yaml" \
  -v "${JCasC_FILE}:/var/jenkins_home/jenkins.yaml:Z" \
  -v "${INIT_GROOVY_FILE}:/var/jenkins_home/init.groovy.d/01-agents.groovy:Z" \
  -v "${JENKINS_VOLUME_NAME}:/var/jenkins_home:Z" \
  "${JENKINS_IMAGE}"

echo "==> Waiting for Jenkins Controller initial boot..."
until podman exec "${JENKINS_CONTAINER_NAME}" curl -s -f http://localhost:8080/login > /dev/null 2>&1; do
  sleep 3
done

# ==========================================
# 6. Plugin Installation & Controller Restart
# ==========================================
echo "==> Installing plugins (Recommended + jms-messaging)..."
podman exec "${JENKINS_CONTAINER_NAME}" jenkins-plugin-cli --plugins \
  cloudbees-folder \
  antisamy-markup-formatter \
  build-timeout \
  credentials-binding \
  timestamper \
  ws-cleanup \
  ant \
  gradle \
  workflow-aggregator \
  github-branch-source \
  pipeline-github-lib \
  pipeline-stage-view \
  git \
  ssh-slaves \
  matrix-auth \
  pam-auth \
  ldap \
  email-ext \
  mailer \
  dark-theme \
  jms-messaging

echo "==> Restarting Jenkins Controller..."
podman restart "${JENKINS_CONTAINER_NAME}"

echo "==> Waiting for Jenkins subsystem readiness post-restart..."
until podman exec "${JENKINS_CONTAINER_NAME}" curl -s -f http://localhost:8080/login > /dev/null 2>&1; do
  sleep 3
done

sleep 4

# ==========================================
# 7. Extract Connection Secrets Directly from Volume
# ==========================================
echo "==> Extracting agent connection secrets..."
SECRETS_OUTPUT=$(podman exec "${JENKINS_CONTAINER_NAME}" cat /var/jenkins_home/secrets.txt 2>/dev/null || true)

SECRET_1=$(echo "${SECRETS_OUTPUT}" | grep -E '^SECRET_1=' | cut -d'=' -f2 | tr -d '\r\n')
SECRET_2=$(echo "${SECRETS_OUTPUT}" | grep -E '^SECRET_2=' | cut -d'=' -f2 | tr -d '\r\n')

if [ -z "${SECRET_1}" ] || [ -z "${SECRET_2}" ]; then
  echo "ERROR: Failed to extract secrets from /var/jenkins_home/secrets.txt"
  echo "Raw output: ${SECRETS_OUTPUT}"
  exit 1
fi

echo "==> Node Secrets extracted successfully:"
echo "    - ${AGENT_1_NAME}: ${SECRET_1:0:8}..."
echo "    - ${AGENT_2_NAME}: ${SECRET_2:0:8}..."

# ==========================================
# 8. Deploy Inbound Agent Containers
# ==========================================
echo "==> Cleaning old agent containers..."
podman rm -f "${AGENT_1_NAME}" "${AGENT_2_NAME}" 2>/dev/null || true

echo "==> Spawning ${AGENT_1_NAME}..."
podman run -d \
  --name "${AGENT_1_NAME}" \
  --network "${NETWORK_NAME}" \
  --dns 8.8.8.8 --dns 1.1.1.1 \
  --restart unless-stopped \
  "${AGENT_IMAGE}" \
  -url "http://${JENKINS_CONTAINER_NAME}:8080" \
  -secret "${SECRET_1}" \
  -name "${AGENT_1_NAME}"

echo "==> Spawning ${AGENT_2_NAME}..."
podman run -d \
  --name "${AGENT_2_NAME}" \
  --network "${NETWORK_NAME}" \
  --dns 8.8.8.8 --dns 1.1.1.1 \
  --restart unless-stopped \
  "${AGENT_IMAGE}" \
  -url "http://${JENKINS_CONTAINER_NAME}:8080" \
  -secret "${SECRET_2}" \
  -name "${AGENT_2_NAME}"

# ==========================================
# Deployment Summary & Manual Action Guide
# ==========================================
echo ""
echo "=========================================================="
echo " Setup Completed Successfully!"
echo "=========================================================="
echo " Jenkins UI     : http://localhost:${JENKINS_HTTP_PORT}"
echo " Jenkins Creds  : ${JENKINS_ADMIN_USER} / ${JENKINS_ADMIN_PASS}"
echo " RabbitMQ Mgmt  : http://localhost:${RABBITMQ_MGMT_PORT}"
echo " RabbitMQ Creds : ${RABBITMQ_USER} / ${RABBITMQ_PASS}"
echo " Connected Nodes: master, ${AGENT_1_NAME}, ${AGENT_2_NAME}"
echo "----------------------------------------------------------"
echo " PLEASE PERFORM THE REMAINING ACTIONS MANUALLY:"
echo "----------------------------------------------------------"
echo " 1. Setup Admin Account:"
echo "    - Go to http://localhost:${JENKINS_HTTP_PORT}"
echo "    - Create your admin user/account (or log in with default credentials)."
echo "    - Leave authorization open (allow any user to do anything for now)."
echo ""
echo " 2. Add Global Secret Text Credentials:"
echo "    - Navigate to: Manage Jenkins -> Credentials -> System -> Global credentials -> Add Credentials"
echo "    - Add the following 3 Secret Text credentials:"
echo "      * ID/Name: amqp-uri"
echo "        Value  : amqp://${RABBITMQ_USER}:${RABBITMQ_PASS}@${RABBITMQ_CONTAINER_NAME}:5672/"
echo "      * ID/Name: jenkins-url"
echo "        Value  : http://${JENKINS_CONTAINER_NAME}:8080"
echo "      * ID/Name: jenkins-username"
echo "        Value  : ${JENKINS_ADMIN_USER}"
echo "      * ID/Name: jenkins-password"
echo "        Value  : ${JENKINS_ADMIN_PASS}"
echo "=========================================================="
