#!/usr/bin/env bash

set -e

TOKEN=$(hack/access_token.sh)

curl \
    -H "Accept: application/vnd.github.v3+json" \
    -H "Authorization: Bearer $TOKEN" \
    $*
