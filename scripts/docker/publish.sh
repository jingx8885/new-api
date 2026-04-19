#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

VERSION_FILE="${VERSION_FILE:-$ROOT_DIR/VERSION}"
IMAGE="${IMAGE:-jingxu888512321/new-api}"
SECONDARY_IMAGE="${SECONDARY_IMAGE:-}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"
PUSH=1
DRY_RUN=0
MODE="release"
VERSION=""
DOCKERFILE="${DOCKERFILE:-$ROOT_DIR/Dockerfile}"
CONTEXT="${CONTEXT:-$ROOT_DIR}"

usage() {
  cat <<'EOF'
Usage:
  bash scripts/docker/publish.sh [options]

Options:
  --version <tag>              Publish a release version, for example v1.2.3
  --mode <release|alpha>       Publish mode, default release
  --image <name>               Primary image, default jingxu888512321/new-api
  --secondary-image <name>     Optional secondary image, for example ghcr.io/org/repo
  --platforms <list>           buildx platforms, default linux/amd64,linux/arm64
  --load                       Build locally without push
  --dry-run                    Print the docker command without executing it
  --help                       Show this help

Examples:
  bash scripts/docker/publish.sh --version v1.2.3
  bash scripts/docker/publish.sh --mode alpha
  bash scripts/docker/publish.sh --version v1.2.3 --secondary-image ghcr.io/owner/repo
EOF
}

log() {
  printf '[docker-publish] %s\n' "$*"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

normalize_image() {
  local value="$1"
  printf '%s' "${value,,}"
}

current_version_file_content() {
  if [[ -f "$VERSION_FILE" ]]; then
    cat "$VERSION_FILE"
  fi
}

restore_version_file() {
  if [[ "$ORIGINAL_VERSION_EXISTS" -eq 1 ]]; then
    printf '%s' "$ORIGINAL_VERSION_CONTENT" >"$VERSION_FILE"
  else
    rm -f "$VERSION_FILE"
  fi
}

run_build() {
  local -a cmd=(
    docker buildx build
    --file "$DOCKERFILE"
    --platform "$PLATFORMS"
  )

  local tag
  for tag in "${ALL_TAGS[@]}"; do
    cmd+=(--tag "$tag")
  done

  if [[ "$PUSH" -eq 1 ]]; then
    cmd+=(--push)
  else
    cmd+=(--load)
  fi

  cmd+=("$CONTEXT")

  printf 'Version: %s\n' "$RESOLVED_VERSION"
  printf 'Mode: %s\n' "$MODE"
  printf 'Platforms: %s\n' "$PLATFORMS"
  printf 'Tags:\n'
  for tag in "${ALL_TAGS[@]}"; do
    printf '  %s\n' "$tag"
  done

  if [[ "$DRY_RUN" -eq 1 ]]; then
    printf 'Command:'
    printf ' %q' "${cmd[@]}"
    printf '\n'
    return
  fi

  "${cmd[@]}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    --mode)
      MODE="${2:-}"
      shift 2
      ;;
    --image)
      IMAGE="${2:-}"
      shift 2
      ;;
    --secondary-image)
      SECONDARY_IMAGE="${2:-}"
      shift 2
      ;;
    --platforms)
      PLATFORMS="${2:-}"
      shift 2
      ;;
    --load)
      PUSH=0
      shift
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ "$MODE" != "release" && "$MODE" != "alpha" ]]; then
  echo "--mode must be release or alpha" >&2
  exit 1
fi

IMAGE="$(normalize_image "$IMAGE")"
if [[ -n "$SECONDARY_IMAGE" ]]; then
  SECONDARY_IMAGE="$(normalize_image "$SECONDARY_IMAGE")"
fi

if [[ -z "$VERSION" ]]; then
  if [[ "$MODE" == "alpha" ]]; then
    require_cmd git
    DATE_VALUE="${DATE_OVERRIDE:-$(date +%Y%m%d)}"
    GIT_SHA="${GIT_SHA_OVERRIDE:-$(git -C "$ROOT_DIR" rev-parse --short HEAD)}"
    VERSION="alpha-${DATE_VALUE}-${GIT_SHA}"
  else
    echo "--version is required in release mode" >&2
    exit 1
  fi
fi

RESOLVED_VERSION="$VERSION"

declare -a PRIMARY_TAGS=()
declare -a SECONDARY_TAGS=()
declare -a ALL_TAGS=()

if [[ "$MODE" == "alpha" ]]; then
  PRIMARY_TAGS=("$IMAGE:alpha" "$IMAGE:$RESOLVED_VERSION")
  if [[ -n "$SECONDARY_IMAGE" ]]; then
    SECONDARY_TAGS=("$SECONDARY_IMAGE:alpha" "$SECONDARY_IMAGE:$RESOLVED_VERSION")
  fi
else
  PRIMARY_TAGS=("$IMAGE:$RESOLVED_VERSION" "$IMAGE:latest")
  if [[ -n "$SECONDARY_IMAGE" ]]; then
    SECONDARY_TAGS=("$SECONDARY_IMAGE:$RESOLVED_VERSION" "$SECONDARY_IMAGE:latest")
  fi
fi

ALL_TAGS=("${PRIMARY_TAGS[@]}")
if ((${#SECONDARY_TAGS[@]} > 0)); then
  ALL_TAGS+=("${SECONDARY_TAGS[@]}")
fi

if [[ "$DRY_RUN" -eq 0 ]]; then
  require_cmd docker
fi
if [[ "$MODE" == "alpha" || "$DRY_RUN" -eq 0 ]]; then
  require_cmd git
fi

ORIGINAL_VERSION_EXISTS=0
ORIGINAL_VERSION_CONTENT=""
if [[ -f "$VERSION_FILE" ]]; then
  ORIGINAL_VERSION_EXISTS=1
  ORIGINAL_VERSION_CONTENT="$(current_version_file_content)"
fi
trap restore_version_file EXIT

printf '%s' "$RESOLVED_VERSION" >"$VERSION_FILE"

log "building image from $DOCKERFILE"
run_build
