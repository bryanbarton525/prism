#!/usr/bin/env python3
"""Sequential Linux/cluster evaluation with durable journals and GPU telemetry."""
import argparse
import json
import os
import subprocess
import time
from pathlib import Path


def snapshot():
    memory = {}
    for line in Path("/proc/meminfo").read_text().splitlines():
        name, value = line.split(":", 1)
        if name in ("MemTotal", "MemAvailable", "SwapFree"):
            memory[name] = int(value.split()[0]) * 1024
    gpu = subprocess.run(["nvidia-smi", "--query-gpu=memory.used,memory.free", "--format=csv,noheader,nounits"], capture_output=True, text=True, check=True)
    used, free = map(int, gpu.stdout.strip().split(","))
    return dict(time=time.time(), host_memory=memory, gpu_used_mib=used, gpu_free_mib=free)


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--prism", default="prism")
    p.add_argument("--engine", choices=["ollama", "sglang"], required=True)
    p.add_argument("--url", required=True)
    p.add_argument("--model", required=True)
    p.add_argument("--suite", type=Path)
    p.add_argument("--repeat", type=int, default=3)
    p.add_argument("--arms", nargs="+", choices=["off", "first", "advisory"], default=["off", "first"])
    p.add_argument("--laya-url", default="https://laya-gpu.iambarton.com")
    p.add_argument("--output-dir", type=Path, required=True)
    args = p.parse_args()
    args.output_dir.mkdir(parents=True, exist_ok=False)
    env = dict(os.environ, PRISM_MODEL_RUNTIME_ENGINE=args.engine,
               PRISM_MODEL_RUNTIME_BASE_URL=args.url, PRISM_MODEL_RUNTIME_API_KEY="",
               PRISM_MODEL_RUNTIME_MODEL="", PRISM_STATE_DIR=str(args.output_dir / "state"))
    identity = subprocess.check_output([args.prism, "version"], text=True)
    metadata = dict(arguments=vars(args), identity=identity, started=time.time(), blocks=[])
    metadata["arguments"] = {k: str(v) if isinstance(v, Path) else v for k, v in vars(args).items()}
    (args.output_dir / "metadata.json").write_text(json.dumps(metadata, indent=2))
    for rep in range(1, args.repeat + 1):
        arms = args.arms if rep % 2 else list(reversed(args.arms))
        for arm in arms:
            name = f"{rep:02d}-{arm}"
            report = args.output_dir / (name + ".json")
            cmd = [args.prism, "eval", "run", "--model", args.model, "--repeat", "1", "--max-tokens", "4096", "--temperature", "0.1", "--timeout", "180s", "--output", str(report)]
            if args.suite:
                cmd += ["--suite", str(args.suite.resolve())]
            if arm != "off":
                cmd += ["--laya-url", args.laya_url, "--laya-mode", arm]
            block = dict(name=name, command=cmd, started=time.time())
            print("Starting", name, flush=True)
            with (args.output_dir / (name + ".log")).open("x") as log, (args.output_dir / (name + ".telemetry.jsonl")).open("x") as telemetry:
                proc = subprocess.Popen(cmd, env=env, stdout=log, stderr=log)
                try:
                    while proc.poll() is None:
                        s = snapshot()
                        telemetry.write(json.dumps(s) + "\n"); telemetry.flush()
                        if s["host_memory"]["MemAvailable"] < 2 * 1024**3 or s["gpu_free_mib"] < 96:
                            raise RuntimeError("Resource guard: under 2GiB host memory or 96MiB GPU free")
                        time.sleep(2)
                finally:
                    if proc.poll() is None:
                        proc.terminate()
                        try:
                            proc.wait(timeout=15)
                        except subprocess.TimeoutExpired:
                            proc.kill(); proc.wait()
                block.update(returncode=proc.returncode, completed=time.time())
            metadata["blocks"].append(block)
            (args.output_dir / "metadata.json").write_text(json.dumps(metadata, indent=2))
            if proc.returncode:
                raise RuntimeError(f"{name} command failed: inspect retained logs/journal")
            subprocess.run([args.prism, "eval", "export", str(report), "--output", str(args.output_dir / (name + "-export"))], check=True, env=env)
            print("Completed", name, flush=True)


if __name__ == "__main__":
    main()
