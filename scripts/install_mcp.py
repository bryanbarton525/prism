import json
import os
import shutil

config_dir = os.path.expanduser('~/.gemini/config')
mcp_file = os.path.join(config_dir, 'mcp.json')

if not os.path.exists(config_dir):
    os.makedirs(config_dir, exist_ok=True)

data = {}
if os.path.exists(mcp_file):
    try:
        with open(mcp_file, 'r') as f:
            data = json.load(f)
    except json.JSONDecodeError:
        data = {}

if "mcpServers" not in data:
    data["mcpServers"] = {}

data["mcpServers"]["prism"] = {
	"command": shutil.which("prism") or "prism",
	"args": ["mcp", "serve"],
    "env": {
        "PRISM_OLLAMA_HOST": "http://127.0.0.1:11434",
        "PRISM_MODEL_RUNTIME_ENGINE": "sglang",
        "PRISM_MODEL_RUNTIME_BASE_URL": "http://sglang.barton.local/v1",
        "PRISM_MODEL_RUNTIME_MODEL": "openai/gpt-oss-20b"
    }
}

with open(mcp_file, 'w') as f:
    json.dump(data, f, indent=2)

print("Successfully installed Prism MCP server. Prefer `prism install --target antigravity` for skills and specialists too.")
