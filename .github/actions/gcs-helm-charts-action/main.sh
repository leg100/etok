#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR=$(dirname -- "$(readlink -f "${BASH_SOURCE[0]}" || realpath "${BASH_SOURCE[0]}")")

main() {
    args=(--path "${INPUT_PATH?Input 'path' is required}")
    args+=(--bucket "${INPUT_BUCKET?Input 'bucket' is required}")

    "$SCRIPT_DIR/gcs.sh" "${args[@]}"
}

main
