# Approach 2: Pessimistic Lock (`FOR UPDATE`)

## Locking Strategy
- Seat selection query uses:
  - `SELECT id FROM seats ... ORDER BY id LIMIT 1 FOR UPDATE`
- The selected candidate row is locked until transaction commit/rollback.
- This ensures only one transaction can claim the same seat row at a time.

## Expected Behavior
- Correctness improves: all seats should be filled.
- Throughput is limited because workers serialize around the hottest first available row.

## Benchmark Result (3 runs)
- Run 1: `success=120`, `assigned=120`, `duration_ms=217`
- Run 2: `success=120`, `assigned=120`, `duration_ms=200`
- Run 3: `success=120`, `assigned=120`, `duration_ms=168`

## Summary
- Average success: `120 / 120` users
- Average assigned seats: `120 / 120`
- Average execution time: `195 ms`
- Correct and safe, but slower under high concurrency due to lock waiting.