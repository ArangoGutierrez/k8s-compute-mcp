#!/usr/bin/env python3
"""Simple Monte Carlo simulation for testing."""
import random
import os
import sys

# Get job completion index from environment
job_index = os.environ.get("JOB_COMPLETION_INDEX", "0")
iterations = 1000

# Simple Monte Carlo: estimate pi
inside_circle = 0
for _ in range(iterations):
    x = random.uniform(-1, 1)
    y = random.uniform(-1, 1)
    if x*x + y*y <= 1:
        inside_circle += 1

pi_estimate = 4 * inside_circle / iterations

# Write result to isolated output file
output_dir = f"/mnt/data/monte-carlo-{job_index}"
os.makedirs(output_dir, exist_ok=True)

with open(f"{output_dir}/result.txt", "w") as f:
    f.write(f"Pi estimate: {pi_estimate}\n")
    f.write(f"Iterations: {iterations}\n")
    f.write(f"Job index: {job_index}\n")

print(f"Job {job_index}: Pi estimate = {pi_estimate}", flush=True)
sys.exit(0)
