#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
DEPLOYMENT=${1:-C}
case "$DEPLOYMENT" in C|D) ;; *) echo "Usage: sh load-tests/run.sh C|D" >&2; exit 2;; esac
PROJECT=${PROJECT:-assignment4-load}
export HTTP_PORT=${HTTP_PORT:-18080}
export HTTPS_PORT=${HTTPS_PORT:-18443}
export DURATION_SECONDS=${DURATION_SECONDS:-300}
REQUEST_COUNTS=${REQUEST_COUNTS:-"1 10 100 1000 5000"}
ENDPOINTS=${ENDPOINTS:-"static aggregate search dynamic"}
RUN_ID=$(date -u +%Y%m%dT%H%M%SZ)
ROOT="$PWD/load-tests/results/$RUN_ID/$DEPLOYMENT"
mkdir -p "$ROOT"
compose() {
  if [ "$DEPLOYMENT" = D ]; then
    docker compose -p "$PROJECT" -f compose.full.yml -f compose.scale.yml "$@"
  else
    docker compose -p "$PROJECT" -f compose.full.yml "$@"
  fi
}
docker compose -p "$PROJECT" -f compose.full.yml -f compose.scale.yml down
compose up -d --build
docker pull grafana/k6:latest
compose config > "$ROOT/compose.yaml"
docker version > "$ROOT/docker-version.txt"
git rev-parse HEAD > "$ROOT/revision.txt"
git diff > "$ROOT/worktree.patch"
docker image inspect grafana/k6:latest > "$ROOT/k6-image.json"
sampler_pid=""
cleanup() {
  if [ -n "$sampler_pid" ]; then
    kill "$sampler_pid" 2>/dev/null || true
    wait "$sampler_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM
for endpoint in $ENDPOINTS; do
  case "$endpoint" in
    static) path=${STATIC_PATH:-/static/style.css};;
    aggregate) path=/books/top-selling;;
    search) path='/search?q=isla';;
    dynamic) path=/books/1;;
    *) echo "Unknown endpoint: $endpoint" >&2; exit 2;;
  esac
  compose exec -T caddy wget -q -O /dev/null --no-check-certificate "https://app.localhost$path"
  for count in $REQUEST_COUNTS; do
    out="$ROOT/$endpoint/$count"
    mkdir -p "$out"
    printf 'deployment=%s\nendpoint=%s\npath=%s\nrequests=%s\nduration_seconds=%s\ncache=warm\n' "$DEPLOYMENT" "$endpoint" "$path" "$count" "$DURATION_SECONDS" > "$out/case.txt"
    ids=$(compose ps --status running -q)
    (
      while :; do
        date -u +%Y-%m-%dT%H:%M:%SZ >> "$out/containers.log"
        docker stats --no-stream --format '{{json .}}' $ids >> "$out/containers.log"
        for id in $ids; do
          printf '\n%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$id" >> "$out/processes.log"
          docker top "$id" -eo pid,ppid,pcpu,rss,nlwp,comm >> "$out/processes.log"
        done
        sleep 5
      done
    ) &
    sampler_pid=$!
    result=0
    docker run --rm --network "${PROJECT}_default" --user "$(id -u):$(id -g)" \
      -v "$PWD/load-tests:/scripts:ro" -v "$out:/results" \
      -e DEPLOYMENT="$DEPLOYMENT" -e ENDPOINT="$endpoint" -e ENDPOINT_PATH="$path" \
      -e REQUEST_COUNT="$count" -e DURATION_SECONDS="$DURATION_SECONDS" \
      grafana/k6:latest run --out json=/results/requests.json \
      --summary-export /results/summary.json /scripts/scenario.js > "$out/k6.log" 2>&1 || result=$?
    cleanup
    sampler_pid=""
    printf '%s\n' "$result" > "$out/exit-code.txt"
    echo "$DEPLOYMENT $endpoint $count: exit=$result results=$out"
    if [ "$result" -ne 0 ]; then cat "$out/k6.log"; exit "$result"; fi
  done
done
