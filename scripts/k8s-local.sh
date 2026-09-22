#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
KIND=${KIND:-./bin/kind}
CLUSTER=assignment4
CONTEXT=kind-assignment4
if ! "$KIND" get clusters | grep -qx "$CLUSTER"; then
  "$KIND" create cluster --name "$CLUSTER" --config k8s/kind.yaml
fi
docker build --target final -t bookreviews:local .
docker build --target edge -t bookreviews-edge:local .
"$KIND" load docker-image --name "$CLUSTER" bookreviews:local bookreviews-edge:local
docker exec assignment4-control-plane sysctl -w vm.max_map_count=262144
kube() { kubectl --context "$CONTEXT" "$@"; }
kube apply -f k8s/namespace.yaml
kube apply -f k8s/pvc.yaml -f k8s/shared-pvcs.yaml -f k8s/configmap.yaml -f k8s/secret.yaml
kube apply -f k8s/redis-deployment.yaml -f k8s/opensearch-deployment.yaml
kube -n book-review-app rollout status deployment/redis --timeout=300s
kube -n book-review-app rollout status deployment/opensearch --timeout=300s
if kube -n book-review-app get deployment book-review-app >/dev/null 2>&1; then
  kube -n book-review-app scale deployment/book-review-app --replicas=0
  kube -n book-review-app wait --for=delete pod -l app.kubernetes.io/name=book-review-app --timeout=120s
fi
kube -n book-review-app delete job bookreviews-bootstrap --ignore-not-found
kube apply -f k8s/bootstrap-job.yaml
kube -n book-review-app wait --for=condition=complete job/bookreviews-bootstrap --timeout=600s
kube apply -f k8s/deployment.yaml -f k8s/service.yaml -f k8s/caddy.yaml
kube -n book-review-app rollout restart deployment/caddy
kube -n book-review-app rollout status deployment/book-review-app --timeout=300s
kube -n book-review-app rollout status deployment/caddy --timeout=300s
kube -n book-review-app get pods
