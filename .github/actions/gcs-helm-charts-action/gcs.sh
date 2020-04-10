#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

show_help() {
cat << EOF
Usage: $(basename "$0") <options>

    -h, --help               Display help
    -p, --path               The chart directory path
    -b, --bucket             The GCS bucket hosting helm charts
EOF
}

main() {
    local path=
    local bucket=
    local version=
    local appVersion=

    parse_command_line "$@"

    local repo_root
    repo_root=$(git rev-parse --show-toplevel)
    pushd "$repo_root" > /dev/null

    latest_tag=$(git describe --abbrev=0 --tags)

    # strip 'v' prefix for chart version
    version=${latest_tag#v}
    appVersion="$latest_tag"

    rm -rf .release-package
    mkdir -p .release-package

    echo "Packaging chart '$path'..."
    helm package "$path" \
        --destination .release-package \
        --dependency-update \
        --app-version "$appVersion" \
        --version "$version" \

    echo "Copying index from bucket to local fs..."
    gsutil cp "gs://$bucket/index.yaml" .release-package/

    echo "Updating index..."
    helm repo index .release-package \
        --merge .release-package/index.yaml \
        --url "https://$bucket.storage.googleapis.com"

    echo "Copying chart and index from local fs to bucket"
    gsutil rsync .release-package "gs://$bucket/"

    popd > /dev/null
}

parse_command_line() {
    while :; do
        case "${1:-}" in
            -h|--help)
                show_help
                exit
                ;;
            -p|--path)
                if [[ -n "${2:-}" ]]; then
                    path="$2"
                    shift
                else
                    echo "ERROR: '-p|--path' cannot be empty." >&2
                    show_help
                    exit 1
                fi
                ;;
            -b|--bucket)
                if [[ -n "${2:-}" ]]; then
                    bucket="$2"
                    shift
                else
                    echo "ERROR: '-b|--bucket' cannot be empty." >&2
                    show_help
                    exit 1
                fi
                ;;
            *)
                break
                ;;
        esac

        shift
    done

    if [[ -z "$path" ]]; then
        echo "ERROR: '-p|--path' is required." >&2
        show_help
        exit 1
    fi

    if [[ -z "$bucket" ]]; then
        echo "ERROR: '-b|--bucket' is required." >&2
        show_help
        exit 1
    fi

    if [[ ! -d "$path" ]]; then
        echo "ERROR: directory $path does not exist" >&2
        exit 1
    fi

    if ! gsutil ls "gs://$bucket" > /dev/null 2>&1; then
        echo "ERROR: bucket gs://$bucket does not exist" >&2
        exit 1
    fi
}

main "$@"
