# Benchmark Run Summary

## Run Metadata

| Field | Value |
| --- | --- |
| Date | 2026-10-29 15:59:28 IST |
| Total Elapsed | 17m0.1s |
| Suite | quick |
| Git Revision | `43f90c9` |
| Worker Count | 3 |
| API URL | `http://localhost:8080` |
| Results Directory | `results\run-20261029-155928` |

## Experiments

| # | Name | Status | Tasks | Completed | Throughput | P50 | P95 | Report |
| ---: | --- | :---: | ---: | ---: | ---: | ---: | ---: | --- |
| 1 | baseline | ✓ | 20 | 19 | 0.16 tasks/s | 1546.4 ms | 2518.7 ms | [report](results\run-20261029-155928/baseline.md) |
| 2 | worker-scaling | ⚠ | 30 | 26 | 0.22 tasks/s | 2031.4 ms | 3515.1 ms | [report](results\run-20261029-155928/worker-scaling.md) |
| 3 | burst | ✓ | 70 | 63 | 0.38 tasks/s | 2566.3 ms | 5086.2 ms | [report](results\run-20261029-155928/burst.md) |
| 4 | workload-profiles | ✓ | 90 | 79 | 0.22 tasks/s | 2040.2 ms | 3547.2 ms | [report](results\run-20261029-155928/workload-profiles.md) |
| 5 | sustained-load | ✓ | 524 | 247 | 0.92 tasks/s | 12891.4 ms | 15013.2 ms | [report](results\run-20261029-155928/sustained-load.md) |

---
*Total elapsed: 17m0.1s*
