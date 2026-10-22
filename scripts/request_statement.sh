#!/bin/sh
set -eu

curl --fail-with-body \
  --request POST \
  --header 'Content-Type: application/json' \
  --data '{
    "statement_id": "stmt-2026-08-player-17",
    "player_id": "player-17",
    "period": "2026-08",
    "assets": {"created": 12, "active": 9},
    "live_events": {"hosted": 2, "peak_viewers": 480},
    "moderation": {"approved": 8, "pending": 3, "rejected": 1}
  }' \
  http://localhost:8080/statements
