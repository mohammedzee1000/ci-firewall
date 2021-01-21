FROM golang:1.12
WORKDIR /app
COPY ./ ./
RUN sh ./scripts/ci-firewall-setup.sh