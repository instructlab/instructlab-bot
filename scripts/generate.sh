#!/bin/bash

VENV_PATH=""
WORK_DIR=""

usage() {
    echo "Usage: $0 [--work-dir PATH] [--venv-path PATH] PR_NUMBER"
    echo
    echo "  --work-dir PATH: Path to the working directory to move into"
    echo "  --venv-path PATH: Path to the virtual environment to activate, relative to the working directory"
    echo "  PR_NUMBER: The number of the pull request to generate data for"
}

if [ $# -lt 1 ]; then
    usage && exit 1
fi

check_work_dir() {
    if [ -n "$WORK_DIR" ]; then
        cd "$WORK_DIR"
    fi
    if [ ! -d taxonomy ]; then
        echo "taxonomy directory not found"
        exit 1
    fi
    if [ ! -f  config.yaml ]; then
        echo "lab config.yaml file not found"
        exit 1
    fi
    if [ -n "$VENV_PATH" ]; then
        if [ ! -d "$VENV_PATH" ]; then
            echo "venv directory not found"
            exit 1
        fi
    fi
    if [ -n "$WORK_DIR" ]; then
        if [ ! -d "$WORK_DIR" ]; then
            echo "work directory not found"
            exit 1
        fi
    fi
}

generate() {
    check_work_dir

    if [ -n "$VENV_PATH" ]; then
        source "$VENV_PATH/bin/activate"
    fi
    cd taxonomy
    git fetch origin "pull/${PR_ID}/head:pr-${PR_ID}"
    git checkout "pr-${PR_ID}"
    cd ..
    OUTPUT_DIR="generate-pr-${PR_ID}"
    mkdir -p "$OUTPUT_DIR"
    lab generate --output-dir "$OUTPUT_DIR"
}

# Parse command line arguments
while [ $# -gt 0 ]; do
    case "$1" in
        --help)
            usage
            exit 0
            ;;
        --venv-path)
            VENV_PATH="$2"
            shift
            ;;
        --work-dir)
            WORK_DIR="$2"
            shift
            ;;
        *)
            if [ -n "$PR_ID" ]; then
                echo "Invalid argument: $1"
                exit 1
            fi
            PR_ID="$1"
            ;;
    esac
    shift
done

if [ -z "$PR_ID" ]; then
    echo "PR_NUMBER is required"
    usage
    exit 1
fi

generate