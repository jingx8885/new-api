#!/usr/bin/env python3
"""
Batch configure new models for new-api channels through the dashboard API.

Environment variables:
  NEW_API_BASE_URL      e.g. http://127.0.0.1:3000
  NEW_API_ACCESS_TOKEN  dashboard access token from /api/user/token
  NEW_API_USER_ID       numeric dashboard user id

Example:
  NEW_API_BASE_URL=http://127.0.0.1:3000 \
  NEW_API_ACCESS_TOKEN=... \
  NEW_API_USER_ID=1 \
  python3 scripts/channel_model_batch_config.py \
    --config scripts/channel_model_batch_config.example.json
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from typing import Any


DEFAULT_TIMEOUT = 120
MAX_PAGE_SIZE = 100


class ApiError(Exception):
    pass


@dataclass
class OperationResult:
    channel_id: int
    channel_name: str
    before_models: list[str]
    requested_models: list[str]
    after_models: list[str]
    added_models: list[str]
    missing_models: list[str]
    changed: bool
    update_success: bool
    update_message: str
    tests: list[dict[str, Any]]


def normalize_base_url(raw: str) -> str:
    raw = raw.strip()
    if not raw:
        raise ValueError("base url is empty")
    return raw.rstrip("/")


def split_models(value: Any) -> list[str]:
    if value is None:
        return []
    if isinstance(value, str):
        parts = value.split(",")
    elif isinstance(value, list):
        parts = []
        for item in value:
            if isinstance(item, str):
                parts.extend(item.split(","))
            else:
                parts.append(str(item))
    else:
        parts = [str(value)]

    seen: set[str] = set()
    models: list[str] = []
    for part in parts:
        model = part.strip()
        if not model or model in seen:
            continue
        seen.add(model)
        models.append(model)
    return models


def join_models(models: list[str]) -> str:
    return ",".join(models)


def to_bool(value: Any) -> bool:
    if isinstance(value, bool):
        return value
    if isinstance(value, str):
        return value.strip().lower() in {"1", "true", "yes", "y", "on"}
    return bool(value)


def merge_models(existing: list[str], requested: list[str], mode: str) -> list[str]:
    if mode == "replace":
        return list(requested)
    if mode != "append":
        raise ValueError(f"unsupported mode: {mode}")

    merged = list(existing)
    seen = set(existing)
    for model in requested:
        if model in seen:
            continue
        seen.add(model)
        merged.append(model)
    return merged


class NewApiClient:
    def __init__(self, base_url: str, access_token: str, user_id: str, timeout: int) -> None:
        self.base_url = normalize_base_url(base_url)
        self.access_token = access_token.strip()
        self.user_id = str(user_id).strip()
        self.timeout = timeout
        if not self.access_token:
            raise ValueError("NEW_API_ACCESS_TOKEN or --access-token is required")
        if not self.user_id:
            raise ValueError("NEW_API_USER_ID or --user-id is required")

    def request(self, method: str, path: str, body: Any | None = None) -> dict[str, Any]:
        url = self.base_url + path
        data = None
        headers = {
            "Authorization": self.access_token,
            "New-Api-User": self.user_id,
            "Accept": "application/json",
        }
        if body is not None:
            data = json.dumps(body, ensure_ascii=False).encode("utf-8")
            headers["Content-Type"] = "application/json"

        req = urllib.request.Request(url, data=data, headers=headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                raw = resp.read().decode("utf-8")
        except urllib.error.HTTPError as err:
            detail = err.read().decode("utf-8", errors="replace")
            raise ApiError(f"{method} {path} failed with HTTP {err.code}: {detail}") from err
        except urllib.error.URLError as err:
            raise ApiError(f"{method} {path} failed: {err.reason}") from err

        try:
            payload = json.loads(raw)
        except json.JSONDecodeError as err:
            raise ApiError(f"{method} {path} returned non-JSON response: {raw[:300]}") from err
        if not isinstance(payload, dict):
            raise ApiError(f"{method} {path} returned unexpected JSON shape")
        return payload

    def api_success(self, method: str, path: str, body: Any | None = None) -> dict[str, Any]:
        payload = self.request(method, path, body)
        if payload.get("success") is not True:
            message = payload.get("message") or "unknown API error"
            raise ApiError(f"{method} {path} failed: {message}")
        return payload

    def get_channel(self, channel_id: int) -> dict[str, Any]:
        payload = self.api_success("GET", f"/api/channel/{channel_id}")
        data = payload.get("data")
        if not isinstance(data, dict):
            raise ApiError(f"GET /api/channel/{channel_id} returned invalid data")
        return data

    def list_channels(self, status: str = "-1", channel_type: int | None = None) -> list[dict[str, Any]]:
        page = 1
        channels: list[dict[str, Any]] = []
        while True:
            params = {
                "p": str(page),
                "page_size": str(MAX_PAGE_SIZE),
                "status": status,
                "id_sort": "true",
            }
            if channel_type is not None:
                params["type"] = str(channel_type)
            path = "/api/channel/?" + urllib.parse.urlencode(params)
            payload = self.api_success("GET", path)
            data = payload.get("data")
            if not isinstance(data, dict):
                raise ApiError("GET /api/channel/ returned invalid data")
            items = data.get("items")
            if not isinstance(items, list):
                raise ApiError("GET /api/channel/ returned invalid items")
            channels.extend(item for item in items if isinstance(item, dict))
            total = int(data.get("total") or len(channels))
            if len(channels) >= total or not items:
                return channels
            page += 1

    def update_channel_models(self, channel: dict[str, Any], models: list[str]) -> dict[str, Any]:
        payload = dict(channel)
        payload["models"] = join_models(models)
        payload.pop("key", None)
        return self.api_success("PUT", "/api/channel/", payload)

    def test_channel(
        self,
        channel_id: int,
        model: str,
        endpoint_type: str,
        stream: bool,
    ) -> dict[str, Any]:
        params = {"model": model}
        if endpoint_type:
            params["endpoint_type"] = endpoint_type
        params["stream"] = "true" if stream else "false"
        path = f"/api/channel/test/{channel_id}?" + urllib.parse.urlencode(params)
        return self.request("GET", path)


def load_config(path: str) -> dict[str, Any]:
    with open(path, "r", encoding="utf-8") as f:
        config = json.load(f)
    if not isinstance(config, dict):
        raise ValueError("config root must be a JSON object")
    if not isinstance(config.get("channels"), list):
        raise ValueError("config must contain a channels array")
    return config


def resolve_channel_ids(client: NewApiClient, item: dict[str, Any]) -> list[int]:
    ids: list[int] = []
    if "id" in item:
        ids.append(int(item["id"]))
    if "ids" in item:
        ids.extend(int(v) for v in item["ids"])
    needs_listing = any(key in item for key in ("tag", "name", "type", "status", "all"))
    if needs_listing:
        status = str(item.get("status", "-1"))
        channel_type = int(item["type"]) if "type" in item else None
        tag = item.get("tag")
        name = item.get("name")
        all_channels = client.list_channels(status=status, channel_type=channel_type)
        for channel in all_channels:
            if tag is not None and channel.get("tag") != tag:
                continue
            if name is not None and channel.get("name") != name:
                continue
            if item.get("all") is not True and tag is None and name is None and channel_type is None:
                continue
            ids.append(int(channel["id"]))

    seen: set[int] = set()
    result: list[int] = []
    for channel_id in ids:
        if channel_id in seen:
            continue
        seen.add(channel_id)
        result.append(channel_id)
    return result


def configure_one_channel(
    client: NewApiClient,
    channel_id: int,
    requested_models: list[str],
    mode: str,
    test_models: list[str],
    endpoint_type: str,
    stream: bool,
    run_tests: bool,
    dry_run: bool,
    test_interval: float,
) -> OperationResult:
    channel = client.get_channel(channel_id)
    before_models = split_models(channel.get("models", ""))
    next_models = merge_models(before_models, requested_models, mode)
    changed = next_models != before_models
    update_success = True
    update_message = "dry-run" if dry_run else "unchanged"

    if changed and not dry_run:
        update_payload = client.update_channel_models(channel, next_models)
        update_message = str(update_payload.get("message") or "updated")
    elif changed:
        update_message = "would update"

    verified_channel = channel if dry_run else client.get_channel(channel_id)
    after_models = next_models if dry_run else split_models(verified_channel.get("models", ""))
    missing_models = [model for model in requested_models if model not in after_models]

    tests: list[dict[str, Any]] = []
    if run_tests and not dry_run:
        for model in test_models:
            started = time.time()
            response = client.test_channel(channel_id, model, endpoint_type, stream)
            tests.append(
                {
                    "model": model,
                    "success": response.get("success") is True,
                    "message": response.get("message", ""),
                    "time": response.get("time"),
                    "error_code": response.get("error_code"),
                    "elapsed": round(time.time() - started, 3),
                }
            )
            if test_interval > 0:
                time.sleep(test_interval)

    return OperationResult(
        channel_id=channel_id,
        channel_name=str(channel.get("name") or ""),
        before_models=before_models,
        requested_models=requested_models,
        after_models=after_models,
        added_models=[model for model in requested_models if model not in before_models],
        missing_models=missing_models,
        changed=changed,
        update_success=update_success,
        update_message=update_message,
        tests=tests,
    )


def print_human_results(results: list[OperationResult]) -> None:
    print("\n验证结果")
    print("=" * 80)
    for result in results:
        update_state = "OK" if result.update_success and not result.missing_models else "FAIL"
        changed = "changed" if result.changed else "unchanged"
        print(f"[{update_state}] channel={result.channel_id} name={result.channel_name} update={changed} message={result.update_message}")
        print(f"  requested: {', '.join(result.requested_models) or '-'}")
        print(f"  added:     {', '.join(result.added_models) or '-'}")
        print(f"  verified:  {'yes' if not result.missing_models else 'no'}")
        if result.missing_models:
            print(f"  missing:   {', '.join(result.missing_models)}")
        if result.tests:
            for test in result.tests:
                state = "PASS" if test["success"] else "FAIL"
                detail = test.get("message") or ""
                api_time = test.get("time")
                print(f"  test[{state}] model={test['model']} api_time={api_time} elapsed={test['elapsed']}s {detail}")
    print("=" * 80)
    failed_updates = sum(1 for result in results if result.missing_models or not result.update_success)
    failed_tests = sum(1 for result in results for test in result.tests if not test["success"])
    print(f"summary: channels={len(results)} failed_updates={failed_updates} failed_tests={failed_tests}")


def result_to_dict(result: OperationResult) -> dict[str, Any]:
    return {
        "channel_id": result.channel_id,
        "channel_name": result.channel_name,
        "before_models": result.before_models,
        "requested_models": result.requested_models,
        "after_models": result.after_models,
        "added_models": result.added_models,
        "missing_models": result.missing_models,
        "changed": result.changed,
        "update_success": result.update_success,
        "update_message": result.update_message,
        "tests": result.tests,
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Batch configure and test new-api channel models.")
    parser.add_argument("--config", required=True, help="JSON config file path")
    parser.add_argument("--base-url", default=os.getenv("NEW_API_BASE_URL", "http://127.0.0.1:3000"))
    parser.add_argument("--access-token", default=os.getenv("NEW_API_ACCESS_TOKEN", ""))
    parser.add_argument("--user-id", default=os.getenv("NEW_API_USER_ID", ""))
    parser.add_argument("--timeout", type=int, default=DEFAULT_TIMEOUT)
    parser.add_argument("--dry-run", action="store_true", help="Resolve and print intended changes without updating or testing")
    parser.add_argument("--no-test", action="store_true", help="Skip channel tests even if config enables them")
    parser.add_argument("--test-interval", type=float, default=0.0, help="Seconds to wait between test requests")
    parser.add_argument("--json-output", help="Write detailed verification results to this JSON file")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    config = load_config(args.config)
    defaults = config.get("defaults") if isinstance(config.get("defaults"), dict) else {}
    client = NewApiClient(args.base_url, args.access_token, args.user_id, args.timeout)

    results: list[OperationResult] = []
    for item in config["channels"]:
        if not isinstance(item, dict):
            raise ValueError("each channels item must be an object")
        requested_models = split_models(item.get("models"))
        if not requested_models:
            raise ValueError(f"channels item has no models: {item}")

        channel_ids = resolve_channel_ids(client, item)
        if not channel_ids:
            raise ValueError(f"channels item matched no channels: {item}")

        mode = str(item.get("mode", defaults.get("mode", "append"))).strip() or "append"
        endpoint_type = str(item.get("endpoint_type", defaults.get("endpoint_type", ""))).strip()
        stream = to_bool(item.get("stream", defaults.get("stream", False)))
        run_tests = to_bool(item.get("test", defaults.get("test", True))) and not args.no_test
        test_models = split_models(item.get("test_models")) or requested_models

        for channel_id in channel_ids:
            try:
                result = configure_one_channel(
                    client=client,
                    channel_id=channel_id,
                    requested_models=requested_models,
                    mode=mode,
                    test_models=test_models,
                    endpoint_type=endpoint_type,
                    stream=stream,
                    run_tests=run_tests,
                    dry_run=args.dry_run,
                    test_interval=args.test_interval,
                )
            except Exception as err:
                result = OperationResult(
                    channel_id=channel_id,
                    channel_name="",
                    before_models=[],
                    requested_models=requested_models,
                    after_models=[],
                    added_models=[],
                    missing_models=requested_models,
                    changed=False,
                    update_success=False,
                    update_message=str(err),
                    tests=[],
                )
            results.append(result)

    print_human_results(results)
    if args.json_output:
        with open(args.json_output, "w", encoding="utf-8") as f:
            json.dump([result_to_dict(result) for result in results], f, ensure_ascii=False, indent=2)
            f.write("\n")

    failed_updates = any(result.missing_models or not result.update_success for result in results)
    failed_tests = any(not test["success"] for result in results for test in result.tests)
    return 1 if failed_updates or failed_tests else 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        raise SystemExit(130)
    except Exception as err:
        print(f"error: {err}", file=sys.stderr)
        raise SystemExit(1)
