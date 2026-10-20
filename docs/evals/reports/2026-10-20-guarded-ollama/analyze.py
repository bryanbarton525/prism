#!/usr/bin/env python3
"""Import validated raw runs; regenerate tables, figures, and this report."""
import argparse
from collections import Counter
import csv
import gzip
import hashlib
import json
from pathlib import Path
import statistics

import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import numpy as np

ROOT = Path(__file__).resolve().parent
DATA = ROOT / 'data'
IMAGES = ROOT / 'images'
MODELS = ['qwen3.5:9b', 'qwen2.5-coder:7b', 'llama3.1:8b']
LABELS = ['Qwen 3.5 9B', 'Qwen 2.5 Coder 7B', 'Llama 3.1 8B', 'Llama 3.1 8B + Laya']
COLORS = ['#2563eb', '#0891b2', '#7c3aed', '#ea580c']


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n')


def csv_file(name, rows):
    with (DATA / name).open('w', newline='') as f:
        writer = csv.DictWriter(f, fieldnames=list(rows[0]), lineterminator="\n")
        writer.writeheader()
        writer.writerows(rows)


def savefig(fig, name):
    fig.savefig(IMAGES / (name + '.png'), dpi=180, bbox_inches='tight', facecolor=fig.get_facecolor())
    fig.savefig(IMAGES / (name + '.svg'), bbox_inches='tight', facecolor=fig.get_facecolor(), metadata={'Date': None})
    svg = IMAGES / (name + '.svg')
    svg.write_text('\n'.join(line.rstrip() for line in svg.read_text().splitlines()) + '\n')
    plt.close(fig)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--baseline-dir', type=Path)
    parser.add_argument('--laya-dir', type=Path)
    args = parser.parse_args()
    if bool(args.baseline_dir) != bool(args.laya_dir):
        parser.error('provide both directories, or neither to regenerate committed data')
    if args.baseline_dir:
        for arm, directory in [('baseline', args.baseline_dir), ('laya', args.laya_dir)]:
            for name in ['metadata.json', 'execution.json']:
                (DATA / f'{arm}-{name}').write_bytes((directory / name).read_bytes())
            for file in sorted(directory.glob('[0-9][0-9].json')):
                (DATA / f'{arm}-{file.name}.gz').write_bytes(gzip.compress(file.read_bytes(), mtime=0))
    baseline_meta = json.loads((DATA / 'baseline-metadata.json').read_text())
    laya_meta = json.loads((DATA / 'laya-metadata.json').read_text())
    baseline_digests = {m['name']: m['digest'] for m in baseline_meta['models']}
    assert laya_meta['models'][0]['digest'] == baseline_digests['llama3.1:8b']
    assert baseline_meta['ollama'] == laya_meta['ollama']
    reports = [json.loads(gzip.decompress((DATA / f'baseline-{i:02d}.json.gz').read_bytes())) for i in range(1, 4)]
    reports += [json.loads(gzip.decompress((DATA / 'laya-01.json.gz').read_bytes()))]
    baseline_exec = json.loads((DATA / 'baseline-execution.json').read_text())
    laya_exec = json.loads((DATA / 'laya-execution.json').read_text())
    executions = baseline_exec + laya_exec
    case_ids = [c['id'] for c in reports[0]['suite']['cases']]
    assert len(case_ids) == 6
    for i, (report, execution) in enumerate(zip(reports, executions)):
        assert report['completed'] and report['repeat'] == 3 and len(report['trials']) == 18
        assert report['models'] == [MODELS[i] if i < 3 else MODELS[2]]
        assert execution['validated'] and execution['unloaded'] and not execution.get('error')
        assert report['suite_hash'] == reports[0]['suite_hash'] and report['generation'] == reports[0]['generation']
        assert report['build'] == reports[0]['build'] and not report['build']['dirty']
        assert bool(report.get('laya_url')) == (i == 3)
    write_json(DATA / 'suite.json', reports[0]['suite'])
    rows, trials, failures = [], [], []
    for i, (report, execution) in enumerate(zip(reports, executions)):
        ts = report['trials']
        checks = [t['grade']['checks'] for t in ts]
        def rate(prefix):
            return sum(all(v for k, v in c.items() if k.startswith(prefix)) for c in checks)
        consistency = []
        for case in case_ids:
            signatures = Counter(t['grade'].get('semantic_signature') for t in ts if t['case_id'] == case and t['grade'].get('semantic_signature'))
            consistency.append(max(signatures.values(), default=0) / 3)
        completion = sum((call.get('response') or {}).get('usage', {}).get('completion_tokens', 0) for t in ts for call in t['calls'])
        times = [t['duration_ms'] / 1000 for t in ts]
        row = {'label': LABELS[i], 'model': report['models'][0], 'laya': i == 3, 'trials': 18,
               'passed': sum(t['grade']['passed'] for t in ts),
               'required_tool_calls': rate('required_tool:'), 'tool_arguments': rate('tool_arguments'),
               'trials_with_tool_calls': sum(bool(t.get('tool_calls')) for t in ts),
               'observed_argument_passes': sum(bool(t.get('tool_calls')) and t['grade']['checks']['tool_arguments'] for t in ts),
               'decision_fields': rate('outcome:'), 'response_schema': rate('response_schema'),
               'consistency': statistics.mean(consistency), 'median_seconds': statistics.median(times),
               'p95_seconds': float(np.percentile(times, 95)), 'completion_tokens': completion,
               'minimum_free_percent': min(s['free_percent'] for s in execution['memory']),
               'peak_resident_gib': max(s['resident_bytes'] for s in execution['memory']) / 1024 ** 3,
               'laya_applied': sum(c.get('laya_applied', False) for c in checks)}
        outcome_failures = [m for t in ts for m in t['grade']['metrics'] if m['name'].startswith('outcome:') and not m['passed']]
        row['missing_outcome_checks'] = sum('present=false' in m['reason'] for m in outcome_failures)
        row['wrong_present_outcome_checks'] = sum('present=true' in m['reason'] for m in outcome_failures)
        row['baseline_eligible'] = row['passed'] == 18 and row['consistency'] == 1
        rows.append(row)
        for t in ts:
            failed = [k for k,v in t['grade']['checks'].items() if not v]
            trials.append({'label': LABELS[i], 'case': t['case_id'], 'repetition': t['repetition'],
                           'passed': t['grade']['passed'], 'seconds': t['duration_ms'] / 1000,
                           'model_calls': len(t['calls']), 'mcp_calls': len(t.get('tool_calls') or []),
                           'failed_checks': ';'.join(failed), 'error': t.get('error', '')})
            for name in failed:
                failures.append({'label': LABELS[i], 'case': t['case_id'], 'repetition': t['repetition'], 'check': name, 'reason': next(m['reason'] for m in t['grade']['metrics'] if m['name'] == name)})
    csv_file('summary.csv', rows)
    csv_file('trials.csv', trials)
    if failures:
        csv_file('failed-checks.csv', failures)
    write_json(DATA / 'summary.json', rows)
    plt.rcParams.update({'svg.hashsalt': 'prism-eval-v1', 'font.family': 'DejaVu Sans', 'axes.spines.top': False, 'axes.spines.right': False,
                         'axes.labelcolor': '#334155', 'text.color': '#0f172a', 'font.size': 11})
    fig, ax = plt.subplots(figsize=(11, 4.8))
    ax.barh(LABELS[::-1], [r['passed'] for r in rows][::-1], color=COLORS[::-1], height=.6)
    for y, row in enumerate(rows[::-1]):
        ax.text(row['passed'] + .25, y, f"{row['passed']}/18", va='center', fontweight='bold')
    ax.set_xlim(0, 20); ax.set_xticks(range(0, 19, 3)); ax.set_xlabel('Trials passing the complete contract')
    ax.set_title('Diagnostic contract outcomes: guarded Ollama collection', loc='left', fontweight='bold', pad=20)
    ax.text(0, -.23, '3 repetitions × 6 cases per arm · Laya arm includes successful-application requirement', transform=ax.transAxes, fontsize=10)
    savefig(fig, 'pass-counts')
    matrix = np.array([[sum(t['grade']['passed'] for t in report['trials'] if t['case_id'] == case) for case in case_ids] for report in reports])
    fig, ax = plt.subplots(figsize=(12, 4.5))
    ax.imshow(matrix, vmin=0, vmax=3, cmap='YlGnBu', aspect='auto')
    ax.set_yticks(range(4), LABELS)
    ax.set_xticks(range(6), [c.replace('-', '\n') for c in case_ids], fontsize=9)
    for y in range(4):
        for x in range(6):
            ax.text(x, y, f'{matrix[y,x]}/3', ha='center', va='center', color='white' if matrix[y,x] >= 2 else '#0f172a', fontweight='bold')
    ax.set_title('Where the full contract passes', loc='left', fontweight='bold', pad=18)
    savefig(fig, 'case-results')
    fig, axes = plt.subplots(1, 2, figsize=(12, 4.8))
    short = ['Qwen 9B', 'Coder 7B', 'Llama 8B', 'Llama + Laya']
    for i, report in enumerate(reports):
        values = [t['duration_ms'] / 1000 for t in report['trials']]
        axes[0].scatter(np.full(18, i) + np.linspace(-.15, .15, 18), values, color=COLORS[i], alpha=.6, s=22)
        axes[0].plot([i-.25, i+.25], [rows[i]['median_seconds']]*2, color='#0f172a', lw=2)
        samples = executions[i]['memory']
        start = samples[0]['at']
        axes[1].plot([(s['at']-start)/60 for s in samples], [s['free_percent'] for s in samples], color=COLORS[i], label=short[i])
    axes[0].set_xticks(range(4), short, rotation=15); axes[0].set_ylabel('Complete trial duration (seconds)')
    axes[0].set_title('Latency: every trial; line = median', loc='left', fontweight='bold')
    axes[1].axhline(40, color='#dc2626', linestyle='--', label='Stop threshold')
    axes[1].set_ylim(0,100); axes[1].set_ylabel('macOS reported memory free (%)'); axes[1].set_xlabel('Minutes since model block began')
    axes[1].set_title('Memory headroom during collection', loc='left', fontweight='bold'); axes[1].legend(fontsize=8, loc='lower right')
    fig.tight_layout(); savefig(fig, 'latency-memory')
    table = '\n'.join(f"| {r['label']} | {r['passed']}/18 | {100*r['consistency']:.1f}% | {r['median_seconds']:.2f}s | {r['peak_resident_gib']:.2f} GiB | {r['minimum_free_percent']}% |" for r in rows)
    dimensions = '\n'.join(f"| {r['label']} | {r['required_tool_calls']}/18 | {r['observed_argument_passes']}/{r['trials_with_tool_calls']} | {r['decision_fields']}/18 | {r['response_schema']}/18 |" for r in rows)
    eligible = ', '.join(r['label'] for r in rows if r['baseline_eligible']) or 'None'
    text = f'''# Diagnostic Ollama evaluation · {reports[0]['build']['version']}

> **Contract conflicts confirmed. Do not use these scores to select a model.**
> The attached skill hard-codes a different server alias from the fixtures, and
> the system envelope omits the root `decision` required by the evaluator.
> These traces diagnose harness/agent integration, not isolated model capability.

Three local models, six synthetic MCP workflows, three repetitions per case,
and a separate Laya-assisted Llama arm. **72 trials are retained, including failures.**
This dataset replaces the unretained collection affected by memory overload;
none of its earlier observations are pooled into these results.

![Full-contract pass counts](images/pass-counts.png)

| Model / arm | Passed | Semantic consistency | Median trial | Peak sampled Ollama resident | Minimum memory free |
|---|---:|---:|---:|---:|---:|
{table}

Baseline-eligible arms under the all-pass/all-consistent policy: **{eligible}**.
No baseline is approved automatically by this report.

## Confirmed harness and instruction conflicts

The skill's read workflow hard-codes `linear`, while this suite exposes `tracker`
and `projects`. The skill is supplied in the system prompt; task instructions
naming fixture servers appear in the user prompt. Qwen Coder's `issue-status`
repetition 1 attempted server `linear`, received an authorization error, and
reported that error. That behavior cannot fairly be attributed solely to model
selection. See [the skill](../../../../skills/linear-issue-management/SKILL.md)
and [prompt assembly](../../../../internal/app/prompt.go).

The same system prompt ends with a JSON envelope containing `summary`, `findings`,
`artifacts`, and `confidence`. It does not define `decision`, while every suite
case requires particular root-level `/decision/...` paths. The task requests an
extra decision object, but there is no unified typed output contract across the
agent, skill, runtime parser, and grader. The runtime parser also does not
preserve `decision`, `linear_actions`, or `next_action` as structured result
fields; they remain only in `raw_output` when emitted. The evaluator checks
that raw string, so it does not establish structured-field delivery to the
orchestrator. See [result parsing](../../../../internal/result/parse.go).
Missing-field failures therefore mix
instruction/contract design with model behavior.

The advisory Laya path runs before the first generative call, but ranks a tool
shortlist and injects suggestions. It does not enforce a selected server/tool or
route subsequent steps. These cases explicitly name the tools and provide tiny
catalogs. They establish delivery of suggestions, not the quality of a Laya-first
MCP decision system. See [the corrective design and test plan](../../../laya-first-routing-plan.md).

## What these scores mean

A full pass requires the expected MCP calls and arguments, valid response schema,
correct declared decision fields, and model identity. The Laya arm additionally
requires a successful Laya recommendation that actually entered the prompt.
Semantic consistency is agreement on declared decision fields, not wording;
missing signatures contribute zero. Agreement is separate from correctness.

| Arm | Required calls | Observed arguments valid | Decision fields | Response schema |
|---|---:|---:|---:|---:|
{dimensions}

The argument column counts only trials that actually called a downstream tool;
missing calls are failures in the required-call column. Fast completion without
retrieving evidence is not a successful latency result.

![Per-case outcomes](images/case-results.png)

The grader checks specific JSON paths. A correct fact in prose or an artifact
can still fail that contract. Treat these as integration and response-format
results, not a ranking of general intelligence or factual knowledge. See
[failed checks](data/failed-checks.csv) and full traces before drawing conclusions. Across all arms, **{sum(r['missing_outcome_checks'] for r in rows)}**
failed outcome checks had missing JSON paths; **{sum(r['wrong_present_outcome_checks'] for r in rows)}**
had present but incorrect values. These are check counts, not trial counts.
Missing fields do not establish that the surrounding prose is factually correct.
For example, Qwen Coder's `empty-search` repetition 1 claimed three issues without
any recorded downstream MCP call; the fixture would have returned zero. Qwen
3.5's `empty-search` repetition 1 correctly described zero results but omitted
the required `decision` object. Both failed, for materially different reasons.

## Laya comparison

Laya ran at `https://laya-gpu.iambarton.com` on the homelab `omarchy` GPU node.
The [health capture](data/laya-health-before.json) identifies the English checkpoint
and revision; the direct probe and [catalog test](data/laya-ranking.log) passed
before collection. Earlier readiness failures remain documented separately.

Actual Laya application: **{rows[3]['laya_applied']}/18 trials**. Complete-contract
passes changed from **{rows[2]['passed']}/18** without Laya to **{rows[3]['passed']}/18**
with Laya. Required-call completion changed from **{rows[2]["required_tool_calls"]}/18**
to **{rows[3]["required_tool_calls"]}/18**, and response-schema validity from
**{rows[2]["response_schema"]}/18** to **{rows[3]["response_schema"]}/18**.
These dimensions improved in this sample while the complete contract did not.
This small, fixed-order comparison does not establish causation or
statistical significance. Laya ranks candidates; it does not generate the answer.
The automatic path uses a lexical shortlist and the production 500 ms budget.
The six-case response suite has small catalogs; large-catalog ranking quality is
not measured by these response trials. The separate live catalog test uses MiniLM.

## Runtime and memory

Host: Apple M5 Pro, 48 GiB unified memory, arm64, macOS 27.0.1 (26A434).
Ollama version: `{baseline_meta["ollama"]["version"]}`. The bundled Linear agent
uses a 6,144-token context budget. Generation limits are shared across arms.

![Latency and memory headroom](images/latency-memory.png)

All trial durations include model loading when applicable, model calls, tool loops,
and local MCP handling; no warmup or retry is excluded. The memory plot uses macOS
`memory_pressure -Q`, with a 40% stop threshold. Ollama resident size is sampled
from `/api/ps`, and capped at 12 GiB; model artifact size is capped at 8 GiB.
These are samples, not a hard OS allocation limit or a peak RSS measurement.
Every model was unloaded after its block. Background activity was not controlled.

## What to do next

Treat this as a failing regression candidate dataset, not an approved baseline.
First clarify and test the output-contract boundary: the suite requires a
root-level `decision`, while many responses follow only the standard envelope.
Preserve this suite and its scores when testing a revised prompt or schema.
Separately address unsupported answers and missed tool calls; changing JSON
format alone cannot fix those. A larger, counterbalanced collection should
follow before making model-switching or Laya performance claims.

## Reproduce

From the repository root, using the installed release CLI:

```sh
python3 evals/run_matrix.py \\
  --model qwen3.5:9b --model qwen2.5-coder:7b --model llama3.1:8b \\
  --repeat 3 --max-tokens 4096 \\
  --output-dir "$HOME/prism-eval-results/guarded-baseline-001"
python3 evals/run_matrix.py \\
  --model llama3.1:8b --repeat 3 --max-tokens 4096 \\
  --laya-url https://laya-gpu.iambarton.com \\
  --output-dir "$HOME/prism-eval-results/guarded-laya-001"
```

Use new output directories. The wrapper refuses existing outputs and resident
models. [Full operational guide](../../../../evals/README.md) covers prerequisites,
cleanup, regression baselines, and the live Laya catalog test. Model tags can
change; match the full digests in [baseline metadata](data/baseline-metadata.json).

Regenerate this report and all figures from the committed compressed traces:

```sh
python3 -m venv "$HOME/prism-eval-results/report-venv"
"$HOME/prism-eval-results/report-venv/bin/pip" install -r docs/evals/reports/2026-10-20-guarded-ollama/requirements.txt
"$HOME/prism-eval-results/report-venv/bin/python" docs/evals/reports/2026-10-20-guarded-ollama/analyze.py
```

To import a fresh run, add `--baseline-dir PATH --laya-dir PATH`. The importer
requires all 72 trials, matching suite, generation settings, and clean build
identity, plus successful CLI export validation and unloading recorded by the runner.
Never mix incomplete collections into the denominator.

## Dataset and limits

- [Summary CSV](data/summary.csv), [per-trial CSV](data/trials.csv), [suite](data/suite.json).
- Full reports: `data/baseline-01.json.gz` through `baseline-03.json.gz`, plus
  `data/laya-01.json.gz`. Decompress a report and use `prism eval export` to
  recompute grades and produce the standard trace/metric dataset.
- [Protocol](data/protocol.json), [hardware](data/hardware.json), [model configuration](data/model-configs.json),
  [release provenance](data/release.json), and [data checksums](data/SHA256SUMS).
- Exact commands, memory samples, validation and cleanup results are retained
  in the two `*-execution.json` files. Model request/response traces retain the
  normalized Prism runtime fields; provider-only hidden reasoning/load timing
  is not available here.
- Temperature 0.1, max output 4096 tokens per model call, 180 seconds per complete
  trial, three repetitions, same bundled agent prompts and suite. No seed set.
- Model order and the Laya arm were not randomized. Three repeats per case are
  a small functional probe, not a robust statistical benchmark.
- Fixtures are synthetic and versioned. Model output redistribution remains
  subject to the applicable model licenses. No private live MCP records were used.
'''
    (ROOT / 'README.md').write_text(text)
    hashes = [f'{hashlib.sha256(f.read_bytes()).hexdigest()}  {f.relative_to(DATA)}' for f in sorted(DATA.rglob('*')) if f.is_file() and f.name != 'SHA256SUMS']
    (DATA / 'SHA256SUMS').write_text('\n'.join(hashes) + '\n')
    print(json.dumps(rows, indent=2))


if __name__ == '__main__':
    main()
