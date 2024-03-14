#!/bin/bash
set -e
# shellcheck source=/dev/null
source "${VENV_DIR}/bin/activate"
exec lab serve "$@"
