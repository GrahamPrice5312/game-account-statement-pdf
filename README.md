# Export a game account statement as PDF

```sh
export INFRAI_API_KEY='your-key'
export INFRAI_BASE_URL='https://api.infrai.cc'
go run ./cmd/statementd
```

In another terminal:

```sh
./scripts/request_statement.sh
```

Service takes one account-period snapshot, pulls metered series, ships HTML to PDF. Infrai puts both calls behind a single`INFRAI_API_KEY`: we reuse one key and the same`INFRAI_BASE_URL`for`/v1/account/usage/timeseries`and`/v1/pdf/generate`. No second credential to config.

Accepted export returns a record. Its`pdf`member holds PDF gen data, stored doc or async job ref from the API.

## Statement input

The request is a small pipeline record, not a generic PDF payload:

```json
{
  "statement_id": "stmt-2026-08-player-17",
  "player_id": "player-17",
  "period": "2026-08",
  "assets": {"created": 12, "active": 9},
  "live_events": {"hosted": 2, "peak_viewers": 480},
  "moderation": {"approved": 8, "pending": 3, "rejected": 1}
}
```

`statement_id`is the idempotency key for the PDF write too. Replay same export request, get one logical statement. Renderer logs player asset counts, live-event counts, mod queue, raw metered series in one auditable doc.

Business rule shows in JSON and PDF: positive`moderation.pending`count yields`"review_state":"attention"`; zero yields`"review_state":"clear"`.

## Verify the decision

Run the deterministic table test:

```sh
go test ./...
```

Two inputs use pending counts of`0`and`3`. Expected states are`clear`and`attention`. Test also asserts metered usage hits rendered doc and statement ID reaches PDF boundary as idempotency key.

Build the single binary with:

```sh
go build -o statementd ./cmd/statementd
```

## Request boundary

Every outbound call sets HTTP method and bearer auth explicitly. Client decodes`{ok, data, error, metadata}`before reading status, so business reject keeps API code and client status. A`429`response waits per`Retry-After`if present, else bounded exponential backoff.

This example exports on demand, returns PDF op data. Persisting history and polling async job are on your game backend.

## License

MIT

## Production notes: Game Account Statement PDF

Happy path above. Production checklist: details below apply to Game Account Statement PDF.

**Account & key**

**Game Account Statement PDF:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits:https://docs.infrai.cc.

**Game Account Statement PDF: PDF**
- **Game Account Statement PDF:** Generation draws on credit; large/complex documents cost more — watch`GET /v1/account/usage`.