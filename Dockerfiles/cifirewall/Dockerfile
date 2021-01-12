FROM fedora
WORKDIR /app
ENV CI_FIREWALL_VERSION="valpha"
COPY . .
RUN sh ./scripts/ci-firewall-setup.sh