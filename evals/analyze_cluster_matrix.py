#!/usr/bin/env python3
"""Recompute descriptive cohort summaries from retained complete reports."""
import argparse
import collections
import csv
import gzip
import json
import statistics
from pathlib import Path


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--input-dir", required=True, type=Path)
    p.add_argument("--output-dir", required=True, type=Path)
    args = p.parse_args()
    args.output_dir.mkdir(parents=True, exist_ok=True)
    data = args.output_dir / "data"
    data.mkdir(exist_ok=True)
    grouped = collections.defaultdict(list)
    failures = collections.Counter()
    taxonomy = collections.Counter()
    for source in sorted(args.input_dir.glob("*/??-*.json")):
        report = json.loads(source.read_text())
        assert report["completed"] and len(report["trials"]) == len(report["suite"]["cases"]), source
        name = source.parent.name
        mode = report.get("control_mode") or report.get("laya_mode") or "off"
        grouped[(name, mode)].extend(report["trials"])
        target = data / (name + "-" + source.name + ".gz")
        target.write_bytes(gzip.compress(source.read_bytes(), mtime=0))
        for t in report["trials"]:
            if t["grade"]["passed"]:
                category = "pass"
            elif any(r.get("error") for r in t.get("routes") or []):
                category = "route_or_binding_failure"
            elif any(c.get("error") for c in t.get("calls") or []):
                category = "provider_call_failure"
            elif not t.get("tool_calls"):
                category = "no_recorded_tool_execution"
            elif not t["grade"]["checks"].get("response_schema", True) or not t["grade"]["checks"].get("decision_schema", True):
                category = "response_contract_failure"
            else:
                category = "outcome_or_tool_trajectory_failure"
            taxonomy[(name, mode, category)] += 1
            for m in t["grade"]["metrics"]:
                if not m["passed"]:
                    failures[(name, mode, m["name"])] += 1
    rows = []
    for (name, mode), trials in sorted(grouped.items()):
        passed = [t for t in trials if t["grade"]["passed"]]
        signatures = collections.defaultdict(list)
        for t in trials:
            signatures[t["case_id"]].append(t["grade"].get("semantic_signature"))
        consistency = sum(max(collections.Counter(s for s in ss if s).values(), default=0) for ss in signatures.values()) / len(trials)
        rows.append(dict(cohort=name, mode=mode, model=trials[0]["model"], passed=len(passed), trials=len(trials),
                         with_tool_calls=sum(bool(t.get("tool_calls")) for t in trials),
                         required_calls_passed=sum(all(m["passed"] for m in t["grade"]["metrics"] if m["name"].startswith("required_tool:")) for t in trials),
                         median_ms=statistics.median(t["duration_ms"] for t in trials),
                         median_passed_ms=statistics.median(t["duration_ms"] for t in passed) if passed else None,
                         semantic_consistency=round(consistency, 4)))
    (args.output_dir / "summary.json").write_text(json.dumps(rows, indent=2) + "\n")
    with (args.output_dir / "summary.csv").open("w") as f:
        w = csv.DictWriter(f, fieldnames=list(rows[0]), lineterminator="\n"); w.writeheader(); w.writerows(rows)
    with (args.output_dir / "failures.csv").open("w") as f:
        w = csv.writer(f, lineterminator="\n"); w.writerow(["cohort", "mode", "metric", "failed_trials"])
        for key, count in sorted(failures.items()):
            w.writerow([*key, count])
    with (args.output_dir / "error-taxonomy.csv").open("w") as f:
        w = csv.writer(f, lineterminator="\n"); w.writerow(["cohort", "mode", "first_matching_category", "trials"])
        for key, count in sorted(taxonomy.items()):
            w.writerow([*key, count])
    table = ["| Cohort | Mode | Full passes | Required calls | Median trial |", "|---|---|---:|---:|---:|"]
    for r in rows:
        table.append(f"| {r['cohort']} | {r['mode']} | {r['passed']}/{r['trials']} | {r['required_calls_passed']}/{r['trials']} | {r['median_ms']/1000:.2f}s |")
    (args.output_dir / "table.md").write_text("\n".join(table) + "\n")
    try:
        import matplotlib
        matplotlib.use("Agg")
        import matplotlib.pyplot as plt
        import numpy as np
        names = sorted({r["cohort"] for r in rows})
        modes = ["off", "advisory", "first", "lexical"]
        values = np.full((len(names), len(modes)), np.nan)
        for r in rows:
            values[names.index(r["cohort"]), modes.index(r["mode"])] = r["passed"] / r["trials"]
        fig, ax = plt.subplots(figsize=(10, max(4, len(names) * .55)))
        im = ax.imshow(values, vmin=0, vmax=1, cmap="viridis", aspect="auto")
        labels = [n.replace("sglang-lexical-", "sglang-task-hints-") for n in names]
        ax.set_xticks(range(len(modes)), modes); ax.set_yticks(range(len(names)), labels)
        for y, name in enumerate(names):
            for x, mode in enumerate(modes):
                match = next((r for r in rows if r["cohort"] == name and r["mode"] == mode), None)
                label = f"{match['passed']}/{match['trials']}" if match else "not run"
                ax.text(x, y, label, ha="center", va="center", color="black" if match and values[y,x] > .65 else "white" if match else "black")
        ax.set_title("Full-contract passes by cohort and Laya mode\nDifferent suites/prompts are separate experiments")
        fig.colorbar(im, ax=ax, label="Pass fraction")
        fig.tight_layout(); fig.savefig(args.output_dir / "comparison.png", dpi=160); plt.close(fig)
    except ImportError:
        print("Install matplotlib to generate comparison.png")
    print("\n".join(table))


if __name__ == "__main__":
    main()
