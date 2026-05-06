# Approach 3: Pessimistic Lock with `SKIP LOCKED`

## Locking Strategy
- Seat selection query uses:
  - `SELECT id FROM seats ... ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED`
- Locked rows are skipped instead of waited on.
- Multiple workers can lock and claim different seats in parallel.

## Expected Behavior
- Maintains correctness (no double-booking).
- Better concurrency and lower latency than plain `FOR UPDATE`.

## Benchmark Result (3 runs)
- Run 1: `success=120`, `assigned=120`, `duration_ms=97`
- Run 2: `success=120`, `assigned=120`, `duration_ms=60`
- Run 3: `success=120`, `assigned=120`, `duration_ms=56`

## Summary
- Average success: `120 / 120` users
- Average assigned seats: `120 / 120`
- Average execution time: `71 ms`
- Fastest approach with full correctness.

## Comparison vs Approach 2
- Both produce correct allocation (`120/120` seats assigned).
- `Approach 3` is about `2.75x` faster on average:
  - Approach 2 avg: `195 ms`
  - Approach 3 avg: `71 ms`
- Main reason: `SKIP LOCKED` avoids lock wait chains and improves parallel seat claiming.