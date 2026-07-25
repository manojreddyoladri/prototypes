#!/usr/bin/env python3
"""Benchmark Neo4j SKIP/LIMIT pagination over Part nodes linked to one Hub."""

from __future__ import annotations

import argparse
import os
import statistics
import time
from pathlib import Path

import matplotlib.pyplot as plt
from neo4j import GraphDatabase

HUB_ID = "assembly-1"
PART_COUNT = 10_000
PAGE_SIZE = 100
ITEM_COUNTS = [100, 1_000, 5_000, 10_000]
SKIP_PROBE = [0, 100, 1_000, 5_000, 9_900]
WARMUP_RUNS = 2
TIMED_RUNS = 5
RESULTS_DIR = Path(__file__).resolve().parent / "results"


def get_driver(uri: str, user: str, password: str):
    return GraphDatabase.driver(uri, auth=(user, password))


def wait_for_neo4j(driver, timeout_sec: float = 120.0) -> None:
    deadline = time.time() + timeout_sec
    last_err: Exception | None = None
    while time.time() < deadline:
        try:
            driver.verify_connectivity()
            return
        except Exception as exc:  # noqa: BLE001 — retry until timeout
            last_err = exc
            time.sleep(1.0)
    raise RuntimeError(f"Neo4j not reachable within {timeout_sec}s") from last_err


def setup_schema(session) -> None:
    session.run("CREATE CONSTRAINT part_idx_unique IF NOT EXISTS FOR (p:Part) REQUIRE p.idx IS UNIQUE")
    session.run("CREATE INDEX hub_id IF NOT EXISTS FOR (h:Hub) ON (h.id)")


def seed_graph(session, part_count: int, reset: bool) -> None:
    if reset:
        session.run("MATCH (n) DETACH DELETE n")

    existing = session.run("MATCH (p:Part) RETURN count(p) AS c").single()
    if existing and existing["c"] >= part_count:
        return

    session.run("MERGE (h:Hub {id: $hub_id})", hub_id=HUB_ID)

    batch = 500
    for start in range(0, part_count, batch):
        end = min(start + batch, part_count)
        session.run(
            """
            MATCH (h:Hub {id: $hub_id})
            UNWIND range($start, $end - 1) AS i
            MERGE (p:Part {idx: i})
            SET p.name = 'Part-' + toString(i)
            MERGE (h)-[:HAS_PART]->(p)
            """,
            hub_id=HUB_ID,
            start=start,
            end=end,
        )


def paginate_parts(session, page_size: int, max_items: int) -> tuple[int, float]:
    """Walk Part nodes with ORDER BY + SKIP/LIMIT; return (rows_read, seconds)."""
    skip = 0
    collected = 0
    t0 = time.perf_counter()
    while collected < max_items:
        limit = min(page_size, max_items - collected)
        rows = list(
            session.run(
                """
                MATCH (h:Hub {id: $hub_id})-[:HAS_PART]->(p:Part)
                RETURN p.idx AS idx
                ORDER BY p.idx
                SKIP $skip LIMIT $limit
                """,
                hub_id=HUB_ID,
                skip=skip,
                limit=limit,
            )
        )
        if not rows:
            break
        collected += len(rows)
        skip += len(rows)
    elapsed = time.perf_counter() - t0
    return collected, elapsed


def fetch_page_at_skip(session, skip: int, limit: int) -> int:
    rows = list(
        session.run(
            """
            MATCH (h:Hub {id: $hub_id})-[:HAS_PART]->(p:Part)
            RETURN p.idx AS idx
            ORDER BY p.idx
            SKIP $skip LIMIT $limit
            """,
            hub_id=HUB_ID,
            skip=skip,
            limit=limit,
        )
    )
    return len(rows)


def median_timed(run_fn, warmup: int, runs: int) -> float:
    for _ in range(warmup):
        run_fn()
    samples = [run_fn() for _ in range(runs)]
    return statistics.median(samples)


def run_benchmarks(session, part_count: int) -> tuple[dict[int, float], dict[int, float]]:
    """Returns (total_time_by_item_count, single_page_ms_by_skip)."""
    total_times: dict[int, float] = {}
    for n in ITEM_COUNTS:
        if n > part_count:
            continue

        def one_run() -> float:
            _, elapsed = paginate_parts(session, PAGE_SIZE, n)
            return elapsed

        total_times[n] = median_timed(one_run, WARMUP_RUNS, TIMED_RUNS)

    page_latencies: dict[int, float] = {}
    for skip in SKIP_PROBE:
        if skip >= part_count:
            continue

        def one_page() -> float:
            t0 = time.perf_counter()
            fetch_page_at_skip(session, skip, PAGE_SIZE)
            return (time.perf_counter() - t0) * 1000.0

        page_latencies[skip] = median_timed(one_page, WARMUP_RUNS, TIMED_RUNS)

    return total_times, page_latencies


def plot_results(
    total_times: dict[int, float],
    page_latencies: dict[int, float],
    out_dir: Path,
    part_count: int,
) -> None:
    out_dir.mkdir(parents=True, exist_ok=True)

    fig, axes = plt.subplots(1, 2, figsize=(12, 5))

    xs = sorted(total_times.keys())
    ys = [total_times[x] for x in xs]
    axes[0].plot(xs, ys, marker="o", linewidth=2, color="#2563eb")
    axes[0].set_title(f"Full pagination cost (page size = {PAGE_SIZE})")
    axes[0].set_xlabel("Total Part rows to read")
    axes[0].set_ylabel("Wall time (seconds)")
    axes[0].set_xscale("log")
    axes[0].grid(True, alpha=0.3)
    for x, y in zip(xs, ys):
        axes[0].annotate(f"{y:.3f}s", (x, y), textcoords="offset points", xytext=(0, 8), ha="center")

    sx = sorted(page_latencies.keys())
    sy = [page_latencies[x] for x in sx]
    axes[1].plot(sx, sy, marker="s", linewidth=2, color="#dc2626")
    axes[1].set_title(f"Single-page latency (LIMIT {PAGE_SIZE})")
    axes[1].set_xlabel("SKIP offset")
    axes[1].set_ylabel("Latency (ms)")
    axes[1].grid(True, alpha=0.3)
    for x, y in zip(sx, sy):
        axes[1].annotate(f"{y:.1f}ms", (x, y), textcoords="offset points", xytext=(0, 8), ha="center")

    fig.suptitle(
        f"Neo4j pagination: 1 Hub → {part_count:,} Part nodes (ORDER BY + SKIP/LIMIT)",
        fontsize=11,
    )
    fig.tight_layout()
    png_path = out_dir / "pagination_benchmark.png"
    fig.savefig(png_path, dpi=150)
    plt.close(fig)

    csv_path = out_dir / "pagination_benchmark.csv"
    with csv_path.open("w", encoding="utf-8") as f:
        f.write("metric,key,value,unit\n")
        for n, t in sorted(total_times.items()):
            f.write(f"full_pagination,{n},{t:.6f},seconds\n")
        for skip, ms in sorted(page_latencies.items()):
            f.write(f"single_page_skip,{skip},{ms:.4f},milliseconds\n")

    print(f"Wrote {png_path}")
    print(f"Wrote {csv_path}")


def print_summary(total_times: dict[int, float], page_latencies: dict[int, float]) -> None:
    print("\n--- Full pagination (page size {}) ---".format(PAGE_SIZE))
    for n in sorted(total_times):
        print(f"  read {n:>6,} parts -> {total_times[n]:.4f}s")

    print("\n--- Single page latency ---")
    for skip in sorted(page_latencies):
        print(f"  SKIP {skip:>6,} -> {page_latencies[skip]:.2f} ms")

    if len(total_times) >= 2:
        keys = sorted(total_times)
        first, last = keys[0], keys[-1]
        ratio = total_times[last] / max(total_times[first], 1e-9)
        print(f"\nTime ratio ({last:,} vs {first:,} rows): {ratio:.1f}x")


def main() -> None:
    parser = argparse.ArgumentParser(description="Benchmark Neo4j Part pagination")
    parser.add_argument("--uri", default=os.environ.get("NEO4J_URI", "bolt://localhost:7687"))
    parser.add_argument("--user", default=os.environ.get("NEO4J_USER", "neo4j"))
    parser.add_argument("--password", default=os.environ.get("NEO4J_PASSWORD", "password"))
    parser.add_argument("--parts", type=int, default=PART_COUNT)
    parser.add_argument("--no-reset", action="store_true", help="Keep existing graph data")
    args = parser.parse_args()
    part_count = args.parts

    driver = get_driver(args.uri, args.user, args.password)
    wait_for_neo4j(driver)

    with driver.session() as session:
        setup_schema(session)
        seed_graph(session, part_count, reset=not args.no_reset)
        total_times, page_latencies = run_benchmarks(session, part_count)

    driver.close()

    print_summary(total_times, page_latencies)
    plot_results(total_times, page_latencies, RESULTS_DIR, part_count)


if __name__ == "__main__":
    main()
