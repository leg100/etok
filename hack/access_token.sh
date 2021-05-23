#!/usr/bin/env bash

# Generate an access token from a JWT for use with the Github API

JWT=$(hack/jwt.rb)

curl \
    -X POST \
    -H "Accept: application/vnd.github.v3+json" \
    -H "Authorization: Bearer $JWT" \
    https://api.github.com/app/installations/${ETOK_INSTALL_ID}/access_tokens
