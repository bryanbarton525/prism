# Pulse-derived tokenizer and model format

`internal/toolmodel/wordpiece.go` and the Potion model-format handling in
`internal/toolmodel/potion.go` and `internal/toolmodel/setup.go` are adapted from
[Pulse](https://github.com/bryanbarton525/pulse), copyright 2026 Bryan Barton.
Pulse is licensed under Apache License 2.0; the license text is in
[LICENSE](LICENSE). The adaptations integrate the code with Prism's local MCP
tool recommendation workflow and its pinned model setup process.
