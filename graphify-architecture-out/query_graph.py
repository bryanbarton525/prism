#!/usr/bin/env python3
"""Vocabulary-aware querying and regression benchmarks for the Prism graph."""

from __future__ import annotations

import argparse
import json
import math
import re
from collections import Counter, deque
from pathlib import Path

ROOT = Path(__file__).resolve().parent
GRAPH_PATH = ROOT / "graph.json"
ALIASES_PATH = ROOT / "query_aliases.json"
ANCHORS_PATH = ROOT / "query_anchors.json"
BENCHMARK_PATH = ROOT / "query_benchmark.json"
VOCAB_PATH = ROOT / "vocabulary.txt"
STOPWORDS = {
    "and", "are", "does", "from", "how", "into", "the", "this", "was",
    "what", "when", "where", "which", "with",
}


def words(text: str) -> list[str]:
    chunks = re.findall(r"[^\W\d_]+", text or "", re.UNICODE)
    result: list[str] = []
    for chunk in chunks:
        parts = re.findall(r"[A-Z]+(?=[A-Z][a-z])|[A-Z]?[a-z]+|[A-Z]+", chunk)
        result.extend(part.lower() for part in (parts or [chunk]) if len(part) >= 3)
    return result


def load_data() -> tuple[dict, dict[str, list[str]], dict[str, list[str]], set[str]]:
    graph = json.loads(GRAPH_PATH.read_text())
    aliases = json.loads(ALIASES_PATH.read_text())
    anchors = json.loads(ANCHORS_PATH.read_text())
    vocab = set(VOCAB_PATH.read_text().splitlines())
    return graph, aliases, anchors, vocab


def normalized_terms(text: str) -> set[str]:
    result = set()
    for token in words(text):
        if token in STOPWORDS:
            continue
        if token.endswith("ies") and len(token) > 5:
            token = token[:-3] + "y"
        elif token.endswith("ing") and len(token) > 6:
            token = token[:-3]
        elif token.endswith("ed") and len(token) > 5:
            token = token[:-2]
        elif token.endswith("s") and len(token) > 4:
            token = token[:-1]
        result.add(token)
    return result


def matched_aliases(question: str, aliases: dict[str, list[str]]) -> list[str]:
    question_terms = normalized_terms(question)
    matches: list[tuple[float, str]] = []
    for phrase in aliases:
        phrase_terms = normalized_terms(phrase)
        overlap = len(question_terms & phrase_terms)
        required = len(phrase_terms) if len(phrase_terms) <= 2 else math.ceil(
            len(phrase_terms) * 0.6
        )
        first_candidates = normalized_terms(words(phrase)[0]) if words(phrase) else set()
        first_term_present = bool(first_candidates & question_terms)
        if phrase in question.lower() or (first_term_present and overlap >= required):
            score = overlap / max(len(phrase_terms), 1)
            matches.append((score, phrase))
    matches.sort(key=lambda item: (-item[0], -len(normalized_terms(item[1])), item[1]))
    return [phrase for _, phrase in matches[:3]]


def expand(
    question: str, aliases: dict[str, list[str]], vocab: set[str]
) -> tuple[list[str], list[str]]:
    selected: list[str] = []
    matches = matched_aliases(question, aliases)

    for phrase in matches:
        selected.extend(aliases[phrase])

    selected.extend(
        token for token in words(question)
        if token in vocab and token not in STOPWORDS
    )

    result: list[str] = []
    for token in selected:
        if token in vocab and token not in result:
            result.append(token)
    return result[:12], matches


def build_indexes(graph: dict) -> tuple[dict[str, dict], dict[str, list[tuple[str, dict]]], Counter]:
    nodes = {node["id"]: node for node in graph["nodes"]}
    adjacency: dict[str, list[tuple[str, dict]]] = {node_id: [] for node_id in nodes}
    document_frequency: Counter = Counter()

    for node in nodes.values():
        document_frequency.update(set(words(node.get("label", ""))))

    for edge in graph.get("links", []):
        source, target = edge["source"], edge["target"]
        if source in adjacency and target in adjacency:
            adjacency[source].append((target, edge))
            adjacency[target].append((source, edge))

    return nodes, adjacency, document_frequency


def rank_nodes(
    nodes: dict[str, dict],
    document_frequency: Counter,
    terms: list[str],
    anchors: list[str],
) -> list[tuple[float, str]]:
    total = max(len(nodes), 1)
    ranked: list[tuple[float, str]] = []

    for node_id, node in nodes.items():
        label_tokens = words(node.get("label", ""))
        source_tokens = words(node.get("source_file", ""))
        label = node.get("label", "").lower()
        score = 0.0
        if node_id in anchors:
            score += 1000.0 - anchors.index(node_id)

        for term in terms:
            idf = math.log((total + 1) / (document_frequency.get(term, 0) + 1)) + 1
            if term in label_tokens:
                score += 3.0 * idf
            elif term in label:
                score += 1.8 * idf
            if term in source_tokens:
                score += 0.7 * idf

        if score:
            ranked.append((score, node_id))

    ranked.sort(key=lambda item: (-item[0], nodes[item[1]].get("label", "")))
    return ranked


def query(question: str, depth: int = 2, limit: int = 30) -> dict:
    graph, aliases, anchor_map, vocab = load_data()
    terms, alias_matches = expand(question, aliases, vocab)
    anchors: list[str] = []
    for phrase in alias_matches:
        for node_id in anchor_map.get(phrase, []):
            if node_id not in anchors:
                anchors.append(node_id)
    nodes, adjacency, document_frequency = build_indexes(graph)
    anchors = [node_id for node_id in anchors if node_id in nodes]
    ranked = rank_nodes(nodes, document_frequency, terms, anchors)
    starts = [node_id for _, node_id in ranked[:3]]

    visited = set(starts)
    queue = deque((node_id, 0) for node_id in starts)
    traversed_edges: list[dict] = []

    while queue and len(visited) < limit:
        node_id, current_depth = queue.popleft()
        if current_depth >= depth:
            continue
        for neighbor, edge in adjacency.get(node_id, []):
            traversed_edges.append(edge)
            if neighbor not in visited:
                visited.add(neighbor)
                queue.append((neighbor, current_depth + 1))
            if len(visited) >= limit:
                break

    ranked_ids = [node_id for _, node_id in ranked]
    ordered = sorted(
        visited,
        key=lambda node_id: (
            ranked_ids.index(node_id) if node_id in ranked_ids else len(ranked_ids),
            nodes[node_id].get("label", ""),
        ),
    )

    return {
        "question": question,
        "expanded_terms": terms,
        "matched_aliases": alias_matches,
        "anchor_nodes": anchors,
        "start_nodes": [
            {
                "id": node_id,
                "label": nodes[node_id].get("label"),
                "source_file": nodes[node_id].get("source_file"),
                "source_location": nodes[node_id].get("source_location"),
            }
            for node_id in starts
        ],
        "nodes": [nodes[node_id] for node_id in ordered[:limit]],
        "edges": traversed_edges,
    }


def run_benchmark() -> dict:
    cases = json.loads(BENCHMARK_PATH.read_text())
    results = []

    for case in cases:
        result = query(case["question"], depth=2, limit=40)
        found_files = {node.get("source_file") for node in result["nodes"]}
        expected = case["expected_files"]
        matched = [path for path in expected if path in found_files]
        results.append(
            {
                "id": case["id"],
                "question": case["question"],
                "expanded_terms": result["expanded_terms"],
                "matched_aliases": result["matched_aliases"],
                "anchor_nodes": result["anchor_nodes"],
                "expected_files": expected,
                "matched_files": matched,
                "pass": bool(matched),
                "full_pass": len(matched) == len(expected),
                "start_nodes": result["start_nodes"],
            }
        )

    passed = sum(item["pass"] for item in results)
    full_passed = sum(item["full_pass"] for item in results)
    return {
        "cases": len(results),
        "passed": passed,
        "full_passed": full_passed,
        "pass_rate": passed / len(results) if results else 0,
        "full_pass_rate": full_passed / len(results) if results else 0,
        "results": results,
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("question", nargs="?")
    parser.add_argument("--benchmark", action="store_true")
    parser.add_argument("--json", action="store_true")
    parser.add_argument("--depth", type=int, default=2)
    parser.add_argument("--limit", type=int, default=30)
    args = parser.parse_args()

    if args.benchmark:
        output = run_benchmark()
    elif args.question:
        output = query(args.question, depth=args.depth, limit=args.limit)
    else:
        parser.error("provide a question or --benchmark")

    if args.json:
        print(json.dumps(output, indent=2))
        return

    if args.benchmark:
        print(
            f"Benchmark: {output['passed']}/{output['cases']} useful hits; "
            f"{output['full_passed']}/{output['cases']} matched every expected file"
        )
        for result in output["results"]:
            status = "PASS" if result["pass"] else "FAIL"
            print(f"{status:4} {result['id']}: {', '.join(result['expanded_terms'])}")
        return

    print(f"Query expanded to (from graph vocab, {len(output['expanded_terms'])} tokens): "
          f"{output['expanded_terms']}")
    for node in output["nodes"]:
        print(
            f"{node.get('label')} [{node.get('source_file')} "
            f"{node.get('source_location') or ''}]"
        )


if __name__ == "__main__":
    main()
