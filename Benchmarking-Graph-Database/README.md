# Graph database pagination benchmark (Neo4j)

Small graph: one `Hub` node linked to **10,000** `Part` nodes via `HAS_PART`. The script walks those parts using **ORDER BY + SKIP/LIMIT** (offset pagination), measures latency at **100, 1,000, 5,000, and 10,000** rows, and plots how cost grows as the skip offset increases.

## Prerequisites

- Docker
- Python 3.10+

## Start Neo4j

From this directory:

```bash
docker compose up -d
```

Wait until Browser is up at http://localhost:7474 (user `neo4j`, password `password`).

## Run the benchmark

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
python benchmark_pagination.py
```

Outputs:

- `results/pagination_benchmark.png` — two charts (full walk vs single-page SKIP latency)
- `results/pagination_benchmark.csv` — raw numbers

## What it measures

1. **Full pagination** — Read 100 / 1k / 5k / 10k parts with a fixed page size of 100. Each page uses a larger `SKIP`, so total time grows faster than linear (classic offset-pagination penalty).

2. **Single-page latency** — One query per SKIP value (0, 100, 1k, 5k, 10k) with `LIMIT 100`. Later pages cost more because Neo4j must scan and discard skipped rows.

## Environment

| Variable        | Default                 |
|----------------|-------------------------|
| `NEO4J_URI`    | `bolt://localhost:7687` |
| `NEO4J_USER`   | `neo4j`                 |
| `NEO4J_PASSWORD` | `password`            |

## Re-seed

Drop and recreate the graph on each run (default). To keep data:

```bash
python benchmark_pagination.py --no-reset
```

Change part count:

```bash
python benchmark_pagination.py --parts 10000
```

## Results (local run, 10,000 Part nodes, page size 100)

Graph: one `Hub` → 10,000 `Part` nodes via `HAS_PART`. Pagination: `ORDER BY p.idx SKIP … LIMIT …`.

### Full pagination (median wall time)

| Parts read | Time (s) | vs 100 rows |
|------------|----------|-------------|
| 100        | 0.013    | 1×          |
| 1,000      | 0.085    | ~6.8×       |
| 5,000      | 0.365    | ~29×        |
| 10,000     | 0.678    | **~54×**    |

Reading **100× more rows** (100 → 10,000) took **~54× longer**, because each page uses a larger `SKIP` and Neo4j repeats scan-and-discard work across round trips. Total cost grows faster than linear (offset-pagination penalty).

### Single-page latency (LIMIT 100)

| SKIP  | Latency (ms) |
|-------|----------------|
| 0     | 6.59           |
| 100   | 6.59           |
| 1,000 | 6.99           |
| 5,000 | 7.48           |
| 9,900 | 5.22           |

At this scale, one page stays ~6–7 ms; degradation shows up mainly when **many pages** are chained (table above). On larger graphs, per-page latency at high `SKIP` usually rises more clearly.

### Artifacts

- `results/pagination_benchmark.png` — charts for full walk vs single-page SKIP
- `results/pagination_benchmark.csv` — raw numbers

### Takeaway

Prefer **keyset (cursor) pagination** (e.g. `WHERE p.idx > $last ORDER BY p.idx LIMIT 100`) instead of increasing `SKIP` for large lists.
