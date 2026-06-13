.PHONY: llm-test llm-contract-sglang llm-contract-vllm

llm-test:
	go test ./internal/llm/...

llm-contract-sglang:
	PRISM_LLM_CONTRACT_BASE_URL=http://localhost:30000/v1 \
	PRISM_LLM_CONTRACT_API_KEY=EMPTY \
	PRISM_LLM_CONTRACT_ENGINE=sglang \
	PRISM_LLM_CONTRACT_MODEL=$${PRISM_LLM_CONTRACT_MODEL:-Qwen/Qwen3-Coder} \
	go test -tags=integration ./internal/llm/...

llm-contract-vllm:
	PRISM_LLM_CONTRACT_BASE_URL=http://localhost:8000/v1 \
	PRISM_LLM_CONTRACT_API_KEY=EMPTY \
	PRISM_LLM_CONTRACT_ENGINE=vllm \
	PRISM_LLM_CONTRACT_MODEL=$${PRISM_LLM_CONTRACT_MODEL:-Qwen/Qwen3-Coder} \
	go test -tags=integration ./internal/llm/...
