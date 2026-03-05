#!/usr/bin/env python3
"""Simple MPI hello world for testing."""
from mpi4py import MPI
import sys

comm = MPI.COMM_WORLD
rank = comm.Get_rank()
size = comm.Get_size()

print(f"Hello from rank {rank} of {size}", flush=True)

# Write to output file
with open("/mnt/data/mpi-output.txt", "a") as f:
    f.write(f"Hello from rank {rank} of {size}\n")

sys.exit(0)
