#!/usr/bin/env python3
"""Aggregate results from Monte Carlo replicas and Fibonacci analysis.

Reads:
  - /mnt/data/monte-carlo-*/result.json (from submit_monte_carlo_batch)
  - /mnt/data/fibonacci/levels.json (from submit_reducer with fibonacci script)

Writes:
  - /mnt/data/analysis-report.json (final combined report)
"""
import glob
import json
import math
import os
import sys


def load_monte_carlo_results():
    """Load and aggregate all Monte Carlo replica results."""
    pattern = "/mnt/data/monte-carlo-*/result.json"
    files = sorted(glob.glob(pattern))

    if not files:
        return None

    all_means = []
    all_medians = []
    all_p5 = []
    all_p95 = []
    all_prob_above_200 = []
    all_prob_below_100 = []
    all_max_dd = []
    total_paths = 0

    for path in files:
        with open(path) as f:
            data = json.load(f)
        all_means.append(data["mean_final_price"])
        all_medians.append(data["median_final_price"])
        all_p5.append(data["percentile_5"])
        all_p95.append(data["percentile_95"])
        all_prob_above_200.append(data["prob_above_200"])
        all_prob_below_100.append(data["prob_below_100"])
        all_max_dd.append(data["max_drawdown_mean"])
        total_paths += data["paths"]

    n = len(all_means)
    mean_price = sum(all_means) / n
    std_of_means = math.sqrt(sum((m - mean_price)**2 for m in all_means) / n)

    return {
        "replicas": n,
        "total_paths": total_paths,
        "mean_final_price": round(mean_price, 2),
        "std_across_replicas": round(std_of_means, 2),
        "confidence_interval_95": [
            round(mean_price - 1.96 * std_of_means, 2),
            round(mean_price + 1.96 * std_of_means, 2),
        ],
        "median_final_price": round(sum(all_medians) / n, 2),
        "percentile_5_avg": round(sum(all_p5) / n, 2),
        "percentile_95_avg": round(sum(all_p95) / n, 2),
        "prob_above_200": round(sum(all_prob_above_200) / n, 4),
        "prob_below_100": round(sum(all_prob_below_100) / n, 4),
        "avg_max_drawdown": round(sum(all_max_dd) / n, 4),
    }


def load_fibonacci_levels():
    """Load Fibonacci analysis results."""
    path = "/mnt/data/fibonacci/levels.json"
    if not os.path.exists(path):
        return None
    with open(path) as f:
        return json.load(f)


def main():
    print("Aggregating analysis results...", flush=True)

    mc = load_monte_carlo_results()
    fib = load_fibonacci_levels()

    report = {"stock": "NVDA", "analysis_type": "quantitative"}

    if mc:
        report["monte_carlo"] = mc
        print(f"  Monte Carlo: {mc['replicas']} replicas, "
              f"{mc['total_paths']} total paths", flush=True)
        print(f"  Mean final price: ${mc['mean_final_price']} "
              f"(95% CI: ${mc['confidence_interval_95'][0]}-"
              f"${mc['confidence_interval_95'][1]})", flush=True)

    if fib:
        report["fibonacci"] = fib
        print(f"  Fibonacci: {len(fib.get('retracement_levels', {}))} "
              f"retracement levels", flush=True)

    if mc and fib:
        # Find which Fibonacci levels fall within the Monte Carlo range
        mc_low = mc["percentile_5_avg"]
        mc_high = mc["percentile_95_avg"]
        relevant_levels = {}
        for label, price in fib.get("retracement_levels", {}).items():
            if mc_low <= price <= mc_high:
                relevant_levels[label] = price
        for label, price in fib.get("extension_levels", {}).items():
            if mc_low <= price <= mc_high:
                relevant_levels[label] = price

        report["combined_analysis"] = {
            "fibonacci_levels_in_mc_range": relevant_levels,
            "mc_range": [mc_low, mc_high],
            "interpretation": (
                f"Monte Carlo simulations suggest NVDA price between "
                f"${mc_low} and ${mc_high} (5th-95th percentile) over 1 year. "
                f"{len(relevant_levels)} Fibonacci levels fall within this range, "
                f"suggesting potential support/resistance zones."
            ),
        }

    output_path = "/mnt/data/analysis-report.json"
    with open(output_path, "w") as f:
        json.dump(report, f, indent=2)

    print(f"\nFinal report written to {output_path}", flush=True)
    print(json.dumps(report, indent=2), flush=True)


if __name__ == "__main__":
    main()
    sys.exit(0)
