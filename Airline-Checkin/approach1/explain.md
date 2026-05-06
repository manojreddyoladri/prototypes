# Approach 1: No Row Locking

## Locking Strategy
- Query picks first unassigned seat using plain `SELECT ... LIMIT 1`.
- No `FOR UPDATE`, so many concurrent transactions can read the same seat before any update commits.
- Update uses `WHERE id = ? AND user_id IS NULL`, so only one update wins and others lose.

## Expected Behavior
- High race contention on the earliest seat rows.
- Many users fail to get a seat even though many seats remain unassigned.

## Benchmark Result (3 runs)
- Run 1: `success=2`, `assigned=2`, `duration_ms=243`
- Run 2: `success=2`, `assigned=2`, `duration_ms=174`
- Run 3: `success=4`, `assigned=4`, `duration_ms=87`

## Summary
- Average success: `2.67 / 120` users
- Average assigned seats: `2.67 / 120`
- Average execution time: `168 ms`
- Correctness is poor because missing row locks causes lost booking opportunities.