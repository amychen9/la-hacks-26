#!/usr/bin/env python3
"""
OCR fallback parser for schedule screenshots.

Reads JSON from stdin:
{
  "imageUrl": "...",
  "timezone": "America/Los_Angeles"
}

Prints JSON to stdout following ParseScreenshotResponse shape:
{
  "schemaVersion": "v1",
  "availability": [...],
  "ifNeeded": [...],
  "confidence": 0.0,
  "warnings": [...]
}
"""

from __future__ import annotations

import io
import json
import re
import sys
from datetime import datetime, timedelta, timezone
from typing import List, Tuple

import pytesseract
import requests
from PIL import Image


TIME_PATTERN = re.compile(
    r"\b(?P<hour>\d{1,2})(?::(?P<minute>\d{2}))?\s*(?P<ampm>[ap]m)?\b",
    re.IGNORECASE,
)


def parse_time_token(token: str) -> Tuple[int, int] | None:
    match = TIME_PATTERN.search(token.strip())
    if not match:
        return None

    hour = int(match.group("hour"))
    minute = int(match.group("minute") or "0")
    ampm = (match.group("ampm") or "").lower()

    if ampm == "pm" and hour != 12:
        hour += 12
    if ampm == "am" and hour == 12:
        hour = 0

    if hour > 23 or minute > 59:
        return None
    return hour, minute


def parse_time_range(line: str) -> Tuple[Tuple[int, int], Tuple[int, int]] | None:
    separators = ["-", "–", "—", "to"]
    for sep in separators:
        if sep in line:
            parts = [p.strip() for p in line.split(sep, 1)]
            if len(parts) != 2:
                continue
            start = parse_time_token(parts[0])
            end = parse_time_token(parts[1])
            if start and end:
                return start, end
    return None


def build_iso_for_next_days(
    slot_index: int, start: Tuple[int, int], end: Tuple[int, int]
) -> Tuple[str, str]:
    # Anchor to upcoming days so output stays deterministic and valid.
    base = datetime.now(timezone.utc).astimezone()
    day = base + timedelta(days=slot_index)

    start_dt = day.replace(hour=start[0], minute=start[1], second=0, microsecond=0)
    end_dt = day.replace(hour=end[0], minute=end[1], second=0, microsecond=0)
    if end_dt <= start_dt:
        end_dt += timedelta(hours=1)

    return start_dt.isoformat(), end_dt.isoformat()


def parse_slots_from_text(text: str) -> List[dict]:
    lines = [ln.strip() for ln in text.splitlines() if ln.strip()]
    slots: List[dict] = []

    for line in lines:
        parsed = parse_time_range(line)
        if not parsed:
            continue
        start_iso, end_iso = build_iso_for_next_days(len(slots) + 1, parsed[0], parsed[1])
        slots.append(
            {
                "startIso": start_iso,
                "endIso": end_iso,
                "confidence": 0.55,
                "sourceText": line[:120],
            }
        )
        if len(slots) >= 6:
            break

    return slots


def main() -> int:
    raw = sys.stdin.read()
    if not raw.strip():
        print(
            json.dumps(
                {
                    "schemaVersion": "v1",
                    "availability": [],
                    "ifNeeded": [],
                    "confidence": 0.0,
                    "warnings": ["No OCR input payload provided."],
                }
            )
        )
        return 0

    payload = json.loads(raw)
    image_url = payload.get("imageUrl", "")
    if not image_url:
        print(
            json.dumps(
                {
                    "schemaVersion": "v1",
                    "availability": [],
                    "ifNeeded": [],
                    "confidence": 0.0,
                    "warnings": ["imageUrl is required for OCR fallback."],
                }
            )
        )
        return 0

    response = requests.get(image_url, timeout=15)
    response.raise_for_status()
    image = Image.open(io.BytesIO(response.content))
    text = pytesseract.image_to_string(image)

    slots = parse_slots_from_text(text)
    warnings: List[str] = []
    if not slots:
        warnings.append("OCR found text but could not confidently detect time ranges.")
    else:
        warnings.append("Parsed via pytesseract fallback. Verify extracted slots.")

    out = {
        "schemaVersion": "v1",
        "availability": slots,
        "ifNeeded": [],
        "confidence": 0.58 if slots else 0.2,
        "warnings": warnings,
    }
    print(json.dumps(out))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:
        print(
            json.dumps(
                {
                    "schemaVersion": "v1",
                    "availability": [],
                    "ifNeeded": [],
                    "confidence": 0.0,
                    "warnings": [f"OCR fallback failed: {exc}"],
                }
            )
        )
        raise SystemExit(0)
