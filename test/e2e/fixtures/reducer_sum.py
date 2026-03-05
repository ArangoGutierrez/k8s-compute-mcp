#!/usr/bin/env python3
"""Simple reducer for aggregating test results."""
import os
import sys

# Read all result files from monte carlo jobs
base_dir = "/mnt/data"
total = 0
count = 0

for entry in os.listdir(base_dir):
    if entry.startswith("monte-carlo-"):
        result_file = os.path.join(base_dir, entry, "result.txt")
        if os.path.exists(result_file):
            with open(result_file, "r") as f:
                for line in f:
                    if line.startswith("Pi estimate:"):
                        pi_val = float(line.split(":")[1].strip())
                        total += pi_val
                        count += 1

if count > 0:
    average = total / count
    output_file = "/mnt/data/reduced-result.txt"
    with open(output_file, "w") as f:
        f.write(f"Average Pi estimate: {average}\n")
        f.write(f"Jobs processed: {count}\n")
    print(f"Reduced {count} jobs: average Pi = {average}", flush=True)
else:
    print("No jobs to reduce", flush=True)
    sys.exit(1)

sys.exit(0)
