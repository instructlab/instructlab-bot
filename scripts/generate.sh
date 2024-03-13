#!/bin/bash

if [ $# -lt 1 ]; then
    echo "Usage: $0 PR_NUMBER"
    exit 1
fi
PR_ID="$1"

check_work_dir() {
    if [ ! -d taxonomy ]; then
        echo "taxonomy directory not found"
        exit 1
    fi
    if [ ! -f  config.yaml ]; then
        echo "lab config.yaml file not found"
        exit 1
    fi
}

check_work_dir

cd taxonomy
git fetch origin "pull/${PR_ID}/head:pr-${PR_ID}"
git checkout "pr-${PR_ID}"
cd ..
OUTPUT_DIR="generate-pr-${PR_ID}"
mkdir -p "$OUTPUT_DIR"
lab generate --output-dir "$OUTPUT_DIR"