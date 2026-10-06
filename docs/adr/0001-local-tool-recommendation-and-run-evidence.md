# Local tool recommendation and run evidence

Prism uses an advisory tool recommender across its authorized tool catalogs. Potion and MiniLM produce local embeddings for candidate retrieval; an optional local Kev server can score a shortlist. The executing agent still chooses and calls tools through its existing authorization checks. This preserves the distinction between finding a tool and permitting its execution.

Large tool results are retained only for their owning run and exposed as bounded previews with opaque read references. This allows agents to inspect evidence without placing every byte in the model context, while keeping the run as the access and cleanup owner. Context budgeting accounts for tool definitions and history and can replace old previews with their references. Recommendations and evidence reads remain bounded so their extra model work is visible in end-to-end latency measurements.

## Amendment: Laya categorical reranking (2026-10-05)

The optional decision adapter now supports Laya English using one categorical
choice question over the top 20 retrieved candidates. The evaluated deployment
matched Kev's first-choice results in a 12-task, two-order smoke sample with
roughly one third of the warm HTTP latency. Reusing Kev's independent binary
questions substantially reduced Laya ranking quality, so an endpoint-only
replacement is unsuitable. See [evaluation](../laya-evaluation.md).

Laya choice probabilities describe relative preference within one candidate set.
Prism reports them as `laya_choice` and preserves the initial order on failures.
Generic decision configuration and a pinned managed CPU installation support
migration while retaining Kev/Jev compatibility. Retrieval, authorization, and
the primary offload model are unchanged. Automatic injection remains opt-in and
bounded by 500 ms; the measured English CPU deployment exceeds that budget at
20 candidates. Long-input behavior and probability calibration need broader
validation before enabling that path by default.
