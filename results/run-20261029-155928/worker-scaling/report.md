## Experiment: worker-scaling

Measure observed performance for the current server worker count.

**Date:** 2026-10-29 16:01:28 IST  
**Duration:** 2m0.0s  

### Environment

| Parameter | Value |
| --- | --- |
| API URL | `http://localhost:8080` |
| Git Revision | `43f90c9` |
| Worker Count | 3 |
| Suite | quick |

### Test Parameters

| Parameter | Value |
| --- | --- |
| Name | worker_scaling |
| Task Count | 30 |
| Submission Concurrency | 5 |
| Polling Concurrency | 10 |
| Poll Interval | 500ms |
| Timeout | 2m0s |
| Failure Probability | 0.10 |
| Repetitions | 1 |
| Reported Worker Count | 3 |
| Min Duration (ms) | 100 |
| Max Duration (ms) | 500 |

### Results

| Rep | Submitted | Accepted | Completed | Failed | Timed Out | Pending | Processing | Elapsed | Sub Rate | Throughput | Avg Latency | P50 | P95 | P99 |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 30 | 30 | 26 | 0 | 4 | 0 | 4 | 2m0.0s | 404.74 tasks/s | 0.22 tasks/s | 1934.3 ms | 2031.4 ms | 3515.1 ms | 3518.6 ms |

### Latency Percentiles

| Run | Observations | Avg | P50 | P95 | P99 |
| --- | ---: | ---: | ---: | ---: | ---: |
| Rep 1 | 26 | 1934.3 ms | 2031.4 ms | 3515.1 ms | 3518.6 ms |

### Task Accounting

```
Accepted(30) = Completed(26) + Failed(0) + Pending(0) + Processing(4) + Unknown(0) [TimedOut=4]  =>  30 = 30  [BALANCED]
```

### Throughput & Submission Rate

| Run | Submission Rate | Completed Throughput |
| --- | ---: | ---: |
| 1 | 404.74 tasks/s | 0.22 tasks/s |

### Warnings

- ⚠ Worker scaling measurement assumes GoTask was started with WORKER_COUNT=3. The runner does not modify server configuration.

### Interpretation Notes

- Latency measurements include polling interval and HTTP overhead
- Handler simulates execution time; does not represent real workload
- 4 tasks timed out — latency statistics represent only completed tasks

