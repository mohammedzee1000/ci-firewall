FROM fedora
WORKDIR /app
ENV CI_FIREWALL_VERSION="valpha"
COPY . .
RUN sh ./ci-firewall-setup.sh