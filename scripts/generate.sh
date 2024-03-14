#!/bin/bash

VENV_DIR=""
WORK_DIR=""
NUM_INSTRUCTIONS=10

usage() {
    echo "Usage: $0 [--work-dir PATH] [--venv-dir PATH] PR_NUMBER"
    echo
    echo "  --work-dir PATH: Path to the working directory to move into"
    echo "  --venv-dir PATH: Path to the virtual environment to activate, relative to the working directory"
    echo "  --num-instructions NUM: The number of instructions to generate (default: ${NUM_INSTRUCTIONS})"
    echo "  PR_NUMBER: The number of the pull request to generate data for"
}

if [ $# -lt 1 ]; then
    usage && exit 1
fi

check_work_dir() {
    if [ -n "$WORK_DIR" ]; then
        cd "$WORK_DIR" || exit 1
    fi
    if [ ! -d taxonomy ]; then
        echo "taxonomy directory not found"
        exit 1
    fi
    if [ ! -f  config.yaml ]; then
        echo "lab config.yaml file not found"
        exit 1
    fi
    if [ -n "$VENV_DIR" ]; then
        if [ ! -d "$VENV_DIR" ]; then
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

    if [ -n "$VENV_DIR" ]; then
        # shellcheck source=/dev/null
        source "$VENV_DIR/bin/activate"
    fi
    cd taxonomy || exit 1
    git fetch origin
    git checkout main
    git branch -D "pr-${PR_ID}" 2>/dev/null
    git fetch origin "pull/${PR_ID}/head:pr-${PR_ID}"
    git checkout "pr-${PR_ID}"
    OUTPUT_DIR="generate-pr-${PR_ID}-$(git rev-parse --short HEAD)"
    cd ..
    mkdir -p "$OUTPUT_DIR"
    lab generate --output-dir "$OUTPUT_DIR" --num-instructions "${NUM_INSTRUCTIONS}"
    aws s3 cp --content-type text/plain --recursive "$OUTPUT_DIR" "s3://instruct-lab-bot/generate/${OUTPUT_DIR}"
    cat << EOF > "$OUTPUT_DIR/index.html"
<!DOCTYPE html>
<html>
<head>
    <title>Generated Data for ${OUTPUT_DIR}</title>
</head>
<body>
    <h1>Generated Data for ${OUTPUT_DIR}</h1>
    <ul>
EOF
    for file in "$OUTPUT_DIR"/*; do
        fname=$(basename "${file}")
        if [ "${fname}" == "index.html" ]; then
            continue
        fi
        URL=$(aws s3 presign --region us-east-2 "s3://instruct-lab-bot/generate/${file}")
        echo "        <li><a href=\"${URL}\">${fname}</a></li>" >> "$OUTPUT_DIR/index.html"
    done
    cat << EOF >> "$OUTPUT_DIR/index.html"
    </ul>
</body>
</html>
EOF
    aws s3 cp "$OUTPUT_DIR/index.html" "s3://instruct-lab-bot/generate/${OUTPUT_DIR}/index.html"
    rm -rf "${OUTPUT_DIR}"
    aws s3 presign --region us-east-2 "s3://instruct-lab-bot/generate/${OUTPUT_DIR}/index.html"
}

# Parse command line arguments
while [ $# -gt 0 ]; do
    case "$1" in
        --help)
            usage
            exit 0
            ;;
        --num-instructions)
            NUM_INSTRUCTIONS="$2"
            shift
            ;;
        --venv-dir)
            VENV_DIR="$2"
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
