#!/usr/bin/env python3
"""Geometric Brownian Motion Monte Carlo simulation for stock price forecasting.

Simulates future stock prices using the GBM model:
  dS = mu*S*dt + sigma*S*dW

Each replica (identified by JOB_COMPLETION_INDEX) runs N_PATHS independent
simulations and writes aggregate statistics to /mnt/data/monte-carlo-{index}/result.json.

Parameters are embedded in the script — modify S0, MU, SIGMA for different stocks.
"""
import json
import math
import os
import random
import sys

# === PARAMETERS (modify for your stock) ===
S0 = 130.0       # Current stock price (NVDA ~$130)
MU = 0.15         # Annual drift (expected return)
SIGMA = 0.45      # Annual volatility
T = 1.0           # Time horizon in years
STEPS = 252       # Trading days per year
N_PATHS = 10000   # Simulation paths per replica
# ============================================


def simulate_gbm(s0, mu, sigma, t, steps, n_paths):
    """Run GBM simulation and return final prices and max drawdowns."""
    dt = t / steps
    final_prices = []
    max_drawdowns = []

    for _ in range(n_paths):
        price = s0
        peak = s0
        max_dd = 0.0

        for _ in range(steps):
            z = random.gauss(0, 1)
            price *= math.exp((mu - 0.5 * sigma**2) * dt + sigma * math.sqrt(dt) * z)
            if price > peak:
                peak = price
            dd = (peak - price) / peak
            if dd > max_dd:
                max_dd = dd

        final_prices.append(price)
        max_drawdowns.append(max_dd)

    return final_prices, max_drawdowns


def main():
    job_index = os.environ.get("JOB_COMPLETION_INDEX", "0")

    print(f"Replica {job_index}: Running {N_PATHS} GBM paths "
          f"(S0={S0}, mu={MU}, sigma={SIGMA})", flush=True)

    final_prices, max_drawdowns = simulate_gbm(S0, MU, SIGMA, T, STEPS, N_PATHS)

    final_prices.sort()
    n = len(final_prices)
    mean = sum(final_prices) / n

    result = {
        "replica": int(job_index),
        "paths": N_PATHS,
        "parameters": {"S0": S0, "mu": MU, "sigma": SIGMA, "T": T, "steps": STEPS},
        "mean_final_price": round(mean, 2),
        "median_final_price": round(final_prices[n // 2], 2),
        "std_final_price": round(
            math.sqrt(sum((p - mean) ** 2 for p in final_prices) / n), 2
        ),
        "percentile_5": round(final_prices[int(n * 0.05)], 2),
        "percentile_25": round(final_prices[int(n * 0.25)], 2),
        "percentile_75": round(final_prices[int(n * 0.75)], 2),
        "percentile_95": round(final_prices[int(n * 0.95)], 2),
        "prob_above_200": round(sum(1 for p in final_prices if p > 200) / n, 4),
        "prob_below_100": round(sum(1 for p in final_prices if p < 100) / n, 4),
        "max_drawdown_mean": round(sum(max_drawdowns) / len(max_drawdowns), 4),
    }

    output_dir = f"/mnt/data/monte-carlo-{job_index}"
    os.makedirs(output_dir, exist_ok=True)

    with open(f"{output_dir}/result.json", "w") as f:
        json.dump(result, f, indent=2)

    print(f"Replica {job_index}: mean=${result['mean_final_price']}, "
          f"P5=${result['percentile_5']}, P95=${result['percentile_95']}", flush=True)


if __name__ == "__main__":
    main()
    sys.exit(0)
