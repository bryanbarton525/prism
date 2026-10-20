#!/usr/bin/env python3
"""Run sequential local Ollama evals with conservative memory limits (macOS)."""
import argparse
import datetime
import hashlib
import fcntl
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import time
import urllib.request

GIB = 1024 ** 3


def api(path, body=None):
    request = urllib.request.Request('http://127.0.0.1:11434/api/' + path,
        data=None if body is None else json.dumps(body).encode(), headers={'Content-Type': 'application/json'})
    with urllib.request.urlopen(request, timeout=10) as response:
        return json.load(response)


def memory_free():
    text = subprocess.check_output(['memory_pressure', '-Q'], text=True, timeout=5)
    match = re.search(r'System-wide memory free percentage:\s*(\d+)%', text)
    if not match:
        raise RuntimeError('Cannot determine memory pressure; refusing inference')
    return int(match.group(1))


def check_memory(free, resident_bytes, minimum_free, maximum_bytes):
    if free < minimum_free or resident_bytes > maximum_bytes:
        raise RuntimeError(f'Memory limit reached: free={free}%, resident={resident_bytes / GIB:.1f} GiB')


def save(path, value):
    temporary = path.with_suffix('.tmp')
    temporary.write_text(json.dumps(value, indent=2) + '\n')
    temporary.replace(path)


def unload(model):
    api('generate', {'model': model, 'keep_alive': 0})
    for _ in range(10):
        if not any(m['name'] == model for m in api('ps')['models']):
            return
        time.sleep(1)
    raise RuntimeError('Model did not unload: ' + model)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--model', action='append', required=True)
    p.add_argument('--output-dir', type=Path, required=True)
    p.add_argument('--prism', default='prism')
    p.add_argument('--suite', type=Path)
    p.add_argument('--laya-url', help='Enable recorded Laya recommendations; requires a CLI with --laya-url')
    p.add_argument('--repeat', type=int, default=3)
    p.add_argument('--max-tokens', type=int, default=1024)
    p.add_argument('--max-model-gib', type=float, default=8, help='Preflight artifact-size cap; not a runtime memory estimate')
    p.add_argument('--max-resident-gib', type=float, default=12)
    p.add_argument('--min-free-percent', type=int, default=40)
    a = p.parse_args()
    def interrupted(signum, frame):
        raise KeyboardInterrupt(f"signal {signum}")
    signal.signal(signal.SIGTERM, interrupted)
    lock_path = Path.home() / ".prism" / "eval-matrix.lock"
    lock_path.parent.mkdir(parents=True, exist_ok=True)
    with lock_path.open("a") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            p.error("another guarded matrix is running")
        if len(set(a.model)) != len(a.model) or not 1 <= a.repeat <= 100:
            p.error('models must be unique and repeat must be 1–100')
        if a.max_model_gib <= 0 or a.max_resident_gib <= 0 or not 1 <= a.min_free_percent <= 100:
            p.error('memory limits must be positive; free percentage must be 1–100')
        # Refuse to evict another session's model or compete with its workload.
        if api('ps')['models']:
            p.error('Ollama already has resident models; close other inference sessions and unload them first')
        tags = {m['name']: m for m in api('tags')['models']}
        for model in a.model:
            if model not in tags or tags[model]['size'] > a.max_model_gib * GIB:
                p.error(f'{model} is missing or exceeds the {a.max_model_gib:g} GiB artifact-size cap')
        check_memory(memory_free(), 0, a.min_free_percent, a.max_resident_gib * GIB)
        a.output_dir.mkdir(parents=True, exist_ok=False)
        root = a.output_dir.resolve()
        version = json.loads(subprocess.check_output([a.prism, 'version', '--json'], text=True))
        save(root / 'metadata.json', {'started': datetime.datetime.now(datetime.timezone.utc).isoformat(),
             'cli': version, 'ollama': api('version'), 'models': [tags[m] for m in a.model],
             'settings': {k: str(v) if isinstance(v, Path) else v for k,v in vars(a).items()},
             'temperature': 0.1, 'trial_timeout': '180s', 'warmup': False, 'retries': 0,
             'order': 'sequential model blocks; unload after every block',
             'memory_guard': 'host memory_pressure and total Ollama resident size, sampled every 2 seconds; not a hard OS limit'})
        if a.suite:
            (root / 'suite.yaml').write_bytes(a.suite.read_bytes())
        env = os.environ.copy()
        env.update(PRISM_MODEL_RUNTIME_ENGINE='ollama', PRISM_MODEL_RUNTIME_BASE_URL='http://127.0.0.1:11434')
        rows = []
        for index, model in enumerate(a.model, 1):
            check_memory(memory_free(), 0, a.min_free_percent, a.max_resident_gib * GIB)
            output = root / f'{index:02d}.json'
            command = [a.prism, 'eval', 'run', '--model', model, '--repeat', str(a.repeat),
                       '--max-tokens', str(a.max_tokens), '--temperature', '0.1', '--timeout', '180s', '--output', str(output)]
            if a.laya_url:
                command += ['--laya-url', a.laya_url]
            if a.suite:
                command += ['--suite', str(root / 'suite.yaml')]
            row = {'model': model, 'command': command, 'memory': []}
            rows.append(row)
            save(root / 'execution.json', rows)
            process = None
            print('START', model, flush=True)
            try:
                with (root / f'{index:02d}.log').open('w') as log:
                    process = subprocess.Popen(command, env=env, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
                    while process.poll() is None:
                        resident = api('ps')['models']
                        free, used = memory_free(), sum(m['size'] for m in resident)
                        row['memory'].append({'at': time.time(), 'free_percent': free, 'resident_bytes': used})
                        save(root / 'execution.json', rows)
                        if any(m['name'] != model for m in resident):
                            raise RuntimeError('Another model loaded during this run; stopping to avoid contention')
                        check_memory(free, used, a.min_free_percent, a.max_resident_gib * GIB)
                        time.sleep(2)
                    if process.returncode:
                        raise RuntimeError(f'eval exited {process.returncode}; retain logs and journal')
                report = json.loads(output.read_text())
                row['passed'] = sum(t['grade']['passed'] for t in report['trials'])
                row['trials'] = len(report['trials'])
                subprocess.run([a.prism, 'eval', 'export', str(output), '--output', str(root / f'{index:02d}-export')], check=True)
                row['validated'] = True
            except BaseException as error:
                row['error'] = str(error) or type(error).__name__
                raise
            finally:
                if process is not None and process.poll() is None:
                    os.killpg(process.pid, signal.SIGTERM)
                    try:
                        process.wait(timeout=5)
                    except subprocess.TimeoutExpired:
                        os.killpg(process.pid, signal.SIGKILL)
                        process.wait()
                try:
                    unload(model)
                    row['unloaded'] = True
                finally:
                    save(root / 'execution.json', rows)
            print('DONE', model, row['passed'], '/', row['trials'], flush=True)
        hashes = [f'{hashlib.sha256(f.read_bytes()).hexdigest()}  {f.relative_to(root)}' for f in sorted(root.rglob('*')) if f.is_file()]
        (root / 'SHA256SUMS').write_text('\n'.join(hashes) + '\n')


if __name__ == '__main__':
    main()
