#!/usr/bin/env python3
"""Rebuild comparison from retained, CLI-validated reports; never rewrite grades."""
import argparse
import collections
import gzip
import json
from pathlib import Path
import statistics
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt

ROOT = Path(__file__).resolve().parent

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--import-dir', action='append', type=Path, default=[])
    args = parser.parse_args()
    data = ROOT / 'data'
    data.mkdir(exist_ok=True)
    for directory in args.import_dir:
        for path in sorted(directory.glob('[0-9][0-9].json')):
            report = json.loads(path.read_text())
            assert report['completed'] and report['grader'] == 'mcp-outcomes-v2'
            target = data / f'{directory.name}-{path.name}.gz'
            target.write_bytes(gzip.compress(path.read_bytes(), mtime=0))
        for name in ['metadata.json', 'execution.json', 'SHA256SUMS']:
            path = directory / name
            if path.exists():
                (data / f'{directory.name}-{name}').write_bytes(path.read_bytes())
    rows = []
    builds = set()
    memory = []
    failures = collections.Counter()
    latencies = []
    for path in sorted(data.glob('*.json.gz'), key=lambda p: ('laya' in p.name, p.name)):
        report = json.loads(gzip.decompress(path.read_bytes()))
        builds.add(report['build'].get('revision', 'unknown'))
        assert report['repeat'] == 1
        trials = report['trials']
        mode = report.get('laya_mode', 'original')
        passed = sum(t['grade']['passed'] for t in trials)
        label = report['models'][0] + '\n' + mode
        rows.append((label, passed, len(trials), report['build']))
        latencies.append([t['duration_ms']/1000 for t in trials])
        for trial in trials:
            for check, ok in trial['grade']['checks'].items():
                if not ok:
                    failures[(label.replace('\n', ' / '), check)] += 1
    assert len(builds) <= 1, 'comparison arms must use the same build'
    for path in data.glob('*-execution.json'):
        for block in json.loads(path.read_text()):
            memory.extend(block['memory'])
    if not rows:
        raise SystemExit('No retained reports')
    fig, ax = plt.subplots(figsize=(9, 4.8), layout='constrained')
    fig.patch.set_facecolor('#f8fafc')
    ax.set_facecolor('#f8fafc')
    bars = ax.bar([r[0] for r in rows], [100*r[1]/r[2] for r in rows], color='#2563eb', width=.55)
    ax.set_ylim(0, 110)
    ax.set_ylabel('Trials passing every check (%)')
    ax.set_title('Response contract validation — synthetic MCP replay', loc='left', weight='bold')
    ax.spines[['top', 'right']].set_visible(False)
    ax.bar_label(bars, labels=[f'{r[1]}/{r[2]}' for r in rows], padding=5)
    fig.savefig(ROOT / 'comparison.png', dpi=180)
    plt.close(fig)
    fig, ax = plt.subplots(figsize=(9, 4.8), layout='constrained')
    ax.boxplot(latencies, tick_labels=[r[0] for r in rows], showmeans=True)
    ax.set_ylabel('Trial duration (seconds), including failures')
    ax.set_title('Observed latency — same fixture suite', loc='left', weight='bold')
    ax.spines[['top', 'right']].set_visible(False)
    fig.savefig(ROOT / 'latency.png', dpi=180)
    plt.close(fig)
    lines = ['# Response contracts and optional routing', '',
        'Single-pass smoke results from the fixed implementation. Laya is optional: original mode',
        'uses the model-driven tool loop without contacting Laya. First mode binds',
        'execution to a Laya-selected authorized server/tool before generation.', '',
        '![All-check pass rates](comparison.png)', '', '![Observed latency](latency.png)', '',
        '| Model and mode | All-check passes |', '|---|---:|']
    lines += [f'| {label.replace(chr(10), " / ")} | {passed}/{total} |' for label, passed, total, _ in rows]
    lines += ['', 'These are small synthetic integration probes, not a ranking-quality benchmark.',
        'Expected answers stay in graders; models receive schemas and replayed tool evidence.',
        'Failures remain in the data. One repetition cannot establish consistency;',
        'no smoke run is an approved baseline. Repeat at least three times before approval.',
        'The earlier 8/18 original-loop run is retained in `diagnostics/`, on revision ee6284f.',
        'It preceded a Laya bridge-interface correction and is excluded from these charts.',
        'The historical v0.1.32 report used different contracts and advisory routing;',
        'its pass rates are not a controlled before/after comparison.', '', '## Failed checks', '',
        '| Model and mode | Check | Failed trials |', '|---|---|---:|']
    lines += [f'| {label} | `{check}` | {count} |' for (label, check), count in sorted(failures.items())]
    lines += ['', '## Laya findings', '',
        'The bound path passed issue lookup, conflicting history, and injection-evidence probes.',
        'It abstained on a valid empty-search task. Access denial was retrieved correctly,',
        'but the model placed `access_denied` in the issue-status field instead of null.',
        'For the linked-project task, Laya selected the issue tool again; the model tried',
        'to switch to the project tool, and the runtime rejected that route bypass.',
        'These failures block baseline approval. They distinguish routing selection,',
        'response semantics, and execution enforcement; the small suite does not establish',
        'general ranking quality. Laya remains optional and is not recommended as a default',
        'configuration based on this smoke run.']
    lines += ['', '## Reproduction and provenance', '',
        'Build revision: `' + next(iter(builds)) + '`.',
        f'Minimum sampled free memory: {min(m["free_percent"] for m in memory)}%; peak Ollama resident memory: {max(m["resident_bytes"] for m in memory)/2**30:.2f} GiB.', '',
        'See [the guarded runner guide](../../../../evals/README.md). Exact commands,',
        'binary revisions, model digests, generation settings, memory samples, and',
        'The agent also has a 60-second per-invocation latency budget.',
        'CLI-validated raw reports are retained in `data/`. Runs use one resident',
        'small model at a time, unload between blocks, and stop at guard thresholds.', '',
        'Exact smoke commands (use a binary built from the revision above):', '',
        '```sh',
        'python3 evals/run_matrix.py --prism /path/to/prism --model qwen3.5:9b --model qwen2.5-coder:7b --model llama3.1:8b --repeat 1 --max-tokens 4096 --output-dir "$HOME/prism-eval-results/new-original"',
        'python3 evals/run_matrix.py --prism /path/to/prism --model llama3.1:8b --repeat 1 --max-tokens 4096 --laya-url https://laya-gpu.iambarton.com --laya-mode first --output-dir "$HOME/prism-eval-results/new-laya"',
        '```', '',
        'Use new output directories, and run these sequentially. For consistency validation,',
        'increase `--repeat` to at least 3; approve a baseline only when all checks pass.', '',
        'Regenerate this page and graphs with:', '', '```sh',
        'python3 -m pip install matplotlib',
        'python3 docs/evals/reports/2026-10-20-contract-routing/analyze.py', '```', '']
    (ROOT / 'README.md').write_text('\n'.join(lines))

if __name__ == '__main__':
    main()
