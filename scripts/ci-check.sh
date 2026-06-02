#!/usr/bin/env bash
set -euo pipefail

go mod verify
go test ./...
go test -tags mock ./internal/benchmark -run TestHomelabReleaseIncident_Mock
go vet ./...
go build -v ./cmd/prism
