#!/usr/bin/env bash
set -euo pipefail

AIR_PACKAGE="github.com/cosmtrek/air"
AIR_CONFIG=".air.toml"

go install "${AIR_PACKAGE}@latest"
go run "${AIR_PACKAGE}" -c "${AIR_CONFIG}" server:start
