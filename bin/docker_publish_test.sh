#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_PATH="$ROOT_DIR/scripts/docker/publish.sh"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_contains() {
  local file="$1"
  local expected="$2"
  if ! grep -Fq "$expected" "$file"; then
    echo "Expected to find: $expected" >&2
    echo "--- output ---" >&2
    cat "$file" >&2
    echo "--------------" >&2
    fail "assert_contains failed"
  fi
}

assert_file_equals() {
  local file="$1"
  local expected="$2"
  local actual
  actual="$(cat "$file")"
  if [[ "$actual" != "$expected" ]]; then
    echo "Expected file $file to equal: $expected" >&2
    echo "Actual: $actual" >&2
    fail "assert_file_equals failed"
  fi
}

test_release_tags_and_version_file() {
  local version_file="$TMP_DIR/release-version.txt"
  printf 'previous-version\n' > "$version_file"

  local output="$TMP_DIR/release.out"
  (
    cd "$ROOT_DIR"
    VERSION_FILE="$version_file" \
      bash "$SCRIPT_PATH" \
      --dry-run \
      --version v1.2.3 \
      --image example/new-api \
      >"$output"
  )

  assert_contains "$output" "example/new-api:v1.2.3"
  assert_contains "$output" "example/new-api:latest"
  assert_contains "$output" "docker buildx build"
  assert_file_equals "$version_file" "previous-version"
}

test_alpha_tags_without_latest() {
  local version_file="$TMP_DIR/alpha-version.txt"
  : > "$version_file"

  local output="$TMP_DIR/alpha.out"
  (
    cd "$ROOT_DIR"
    VERSION_FILE="$version_file" \
      DATE_OVERRIDE=20260317 \
      GIT_SHA_OVERRIDE=deadbee \
      bash "$SCRIPT_PATH" \
      --dry-run \
      --mode alpha \
      --image example/new-api \
      >"$output"
  )

  assert_contains "$output" "example/new-api:alpha"
  assert_contains "$output" "example/new-api:alpha-20260317-deadbee"
  if grep -Fq "example/new-api:latest" "$output"; then
    fail "alpha mode should not tag latest"
  fi
  assert_file_equals "$version_file" ""
}

test_secondary_registry_tags() {
  local version_file="$TMP_DIR/secondary-version.txt"
  : > "$version_file"

  local output="$TMP_DIR/secondary.out"
  (
    cd "$ROOT_DIR"
    VERSION_FILE="$version_file" \
      bash "$SCRIPT_PATH" \
      --dry-run \
      --version v2.0.0 \
      --image example/new-api \
      --secondary-image ghcr.io/QuantumNous/New-API \
      >"$output"
  )

  assert_contains "$output" "example/new-api:v2.0.0"
  assert_contains "$output" "ghcr.io/quantumnous/new-api:v2.0.0"
  assert_contains "$output" "ghcr.io/quantumnous/new-api:latest"
}

test_release_tags_and_version_file
test_alpha_tags_without_latest
test_secondary_registry_tags

echo "PASS"
