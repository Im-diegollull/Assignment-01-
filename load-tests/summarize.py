#!/usr/bin/env python3
"""Resume una corrida de load-tests/run.sh en tablas Markdown por endpoint.

Uso: python3 load-tests/summarize.py load-tests/results/<RUN_ID_C> load-tests/results/<RUN_ID_D>
(cada argumento es un directorio de corrida; dentro busca C/ y/o D/). Escribe CSV
en load-tests/summary.csv e imprime las tablas en stdout.
"""
import csv
import json
import re
import sys
from collections import Counter, defaultdict
from pathlib import Path

ENDPOINTS = ["static", "aggregate", "search", "dynamic"]
COUNTS = [1, 10, 100, 1000, 5000]
UNITS = {"B": 1 / 2**20, "KiB": 1 / 1024, "kB": 1 / 1024, "MiB": 1, "MB": 1, "GiB": 1024, "GB": 1024}


def mib(value):
    m = re.match(r"([\d.]+)\s*([A-Za-z]+)", value.split("/")[0].strip())
    return float(m.group(1)) * UNITS.get(m.group(2), 1) if m else 0.0


def service(name):
    # assignment4-load-app2-1 -> app2
    return re.sub(r"^assignment4-load-|-\d+$", "", name)


def case(path):
    summary = json.loads((path / "summary.json").read_text())["metrics"]
    dur = summary["http_req_duration"]
    statuses = Counter()
    with open(path / "requests.json") as f:
        for line in f:
            if '"response_status"' in line and '"Point"' in line:
                statuses[json.loads(line)["data"]["tags"].get("status", "?")] += 1
    peaks = defaultdict(lambda: {"cpu": 0.0, "mem": 0.0, "pids": 0})
    for line in (path / "containers.log").read_text().splitlines():
        if not line.startswith("{"):
            continue
        s = json.loads(line)
        p = peaks[service(s["Name"])]
        p["cpu"] = max(p["cpu"], float(s["CPUPerc"].rstrip("%") or 0))
        p["mem"] = max(p["mem"], mib(s["MemUsage"]))
        p["pids"] = max(p["pids"], int(s["PIDs"] or 0))
    return {
        "reqs": summary["http_reqs"]["count"],
        "avg": dur["avg"], "p50": dur["med"], "p95": dur["p(95)"], "max": dur["max"],
        "status": " ".join(f"{k}:{v}" for k, v in sorted(statuses.items())),
        "peaks": dict(peaks),
    }


def main(dirs):
    rows = []
    for d in map(Path, dirs):
        for dep in ("C", "D"):
            for ep in ENDPOINTS:
                for n in COUNTS:
                    p = d / dep / ep / str(n)
                    if (p / "summary.json").exists():
                        rows.append({"deployment": dep, "endpoint": ep, "n": n, **case(p)})
    with open("load-tests/summary.csv", "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["deployment", "endpoint", "requests", "avg_ms", "p50_ms", "p95_ms", "max_ms", "status",
                    "service", "peak_cpu_pct", "peak_mem_mib", "peak_threads"])
        for r in rows:
            for svc, pk in sorted(r["peaks"].items()):
                w.writerow([r["deployment"], r["endpoint"], r["n"], f"{r['avg']:.2f}", f"{r['p50']:.2f}",
                            f"{r['p95']:.2f}", f"{r['max']:.2f}", r["status"], svc,
                            f"{pk['cpu']:.1f}", f"{pk['mem']:.1f}", pk["pids"]])
    for ep in ENDPOINTS:
        sel = [r for r in rows if r["endpoint"] == ep]
        if not sel:
            continue
        print(f"\n### {ep}\n")
        print("| Dep | Req | avg ms | p50 ms | p95 ms | max ms | status | CPU% pico app(s) | CPU% caddy | CPU% opensearch |")
        print("|---|---|---|---|---|---|---|---|---|---|")
        for r in sorted(sel, key=lambda r: (r["n"], r["deployment"])):
            pk = r["peaks"]
            apps = "/".join(f"{pk[s]['cpu']:.0f}" for s in ("app", "app2", "app3") if s in pk)
            get = lambda s: f"{pk[s]['cpu']:.0f}" if s in pk else "-"
            print(f"| {r['deployment']} | {r['n']} | {r['avg']:.1f} | {r['p50']:.1f} | {r['p95']:.1f} | "
                  f"{r['max']:.1f} | {r['status']} | {apps} | {get('caddy')} | {get('opensearch')} |")


if __name__ == "__main__":
    main(sys.argv[1:] or sorted(str(p) for p in Path("load-tests/results").iterdir()))
