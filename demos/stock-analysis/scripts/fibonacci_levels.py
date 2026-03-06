#!/usr/bin/env python3
"""Fibonacci retracement and extension level calculator.

Computes key Fibonacci levels between a price high and low.
These levels are commonly used as support/resistance zones in technical analysis.

Outputs to /mnt/data/fibonacci/levels.json.
Modify HIGH and LOW constants for your stock's price range.
"""
import json
import os
import sys

# === PARAMETERS (modify for your stock) ===
HIGH = 150.0   # 52-week high (NVDA)
LOW = 100.0    # 52-week low (NVDA)
# ============================================

RETRACEMENT_RATIOS = [0.0, 0.236, 0.382, 0.5, 0.618, 0.786, 1.0]
EXTENSION_RATIOS = [1.0, 1.272, 1.618, 2.0, 2.618]


def compute_levels(high, low):
    """Compute Fibonacci retracement and extension levels."""
    diff = high - low

    retracements = {}
    for r in RETRACEMENT_RATIOS:
        label = f"{r*100:.1f}%"
        retracements[label] = round(high - diff * r, 2)

    extensions = {}
    for r in EXTENSION_RATIOS:
        label = f"{r*100:.1f}%"
        extensions[label] = round(high + diff * (r - 1), 2)

    return retracements, extensions


def main():
    print(f"Computing Fibonacci levels: HIGH={HIGH}, LOW={LOW}", flush=True)

    retracements, extensions = compute_levels(HIGH, LOW)

    result = {
        "high": HIGH,
        "low": LOW,
        "retracement_levels": retracements,
        "extension_levels": extensions,
        "key_support": [retracements["61.8%"], retracements["50.0%"], retracements["38.2%"]],
        "key_resistance": [extensions["161.8%"], extensions["261.8%"]],
    }

    output_dir = "/mnt/data/fibonacci"
    os.makedirs(output_dir, exist_ok=True)

    with open(f"{output_dir}/levels.json", "w") as f:
        json.dump(result, f, indent=2)

    print(f"Fibonacci levels written to {output_dir}/levels.json", flush=True)
    for label, price in retracements.items():
        print(f"  Retracement {label}: ${price}", flush=True)


if __name__ == "__main__":
    main()
    sys.exit(0)
