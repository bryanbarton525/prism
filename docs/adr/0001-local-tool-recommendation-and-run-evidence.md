# Local tool recommendation and run evidence

Prism uses an advisory tool recommender across its authorized tool catalogs. Potion and MiniLM produce local embeddings for candidate retrieval; an optional local Kev server can score a shortlist. The executing agent still chooses and calls tools through its existing authorization checks. This preserves the distinction between finding a tool and permitting its execution.

Large tool results are retained only for their owning run and exposed as bounded previews with opaque read references. This allows agents to inspect evidence without placing every byte in the model context, while keeping the run as the access and cleanup owner. Context budgeting accounts for tool definitions and history and can replace old previews with their references. Recommendations and evidence reads remain bounded so their extra model work is visible in end-to-end latency measurements.
