#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p validation-results
compose() { docker compose -p assignment4 -f compose.full.yml -f compose.scale.yml "$@"; }
compose cp caddy:/data/caddy/pki/authorities/local/root.crt validation-results/compose-root.crt
go run ./cmd/verify -ca validation-results/compose-root.crt > validation-results/compose-three.json
curl --fail --silent --show-error --cacert validation-results/compose-root.crt \
  --resolve app.localhost:443:127.0.0.1 -c validation-results/cookies.txt \
  https://app.localhost/books/1 -o /dev/null
container=$(compose ps -aq app2)
restore() { docker start "$container" >/dev/null; }
trap restore EXIT INT TERM
compose stop app2
curl --fail --silent --show-error --cacert validation-results/compose-root.crt \
  --resolve app.localhost:443:127.0.0.1 -b validation-results/cookies.txt \
  -D validation-results/session-after-stop.headers https://app.localhost/recent -o /dev/null
grep -qi '^location: /books/1' validation-results/session-after-stop.headers
go run ./cmd/verify -instances 2 -ca validation-results/compose-root.crt > validation-results/compose-two.json
restore
trap - EXIT INT TERM
compose ps
