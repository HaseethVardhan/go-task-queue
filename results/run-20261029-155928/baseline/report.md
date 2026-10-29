## Experiment: baseline

Establish baseline performance reference under mixed workload.

**Date:** 2026-10-29 15:59:28 IST  
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
| Name | baseline |
| Task Count | 20 |
| Submission Concurrency | 5 |
| Polling Concurrency | 10 |
| Poll Interval | 500ms |
| Timeout | 2m0s |
| Failure Probability | 0.10 |
| Repetitions | 1 |
| Min Duration (ms) | 100 |
| Max Duration (ms) | 500 |

### Results

| Rep | Submitted | Accepted | Completed | Failed | Timed Out | Pending | Processing | Elapsed | Sub Rate | Throughput | Avg Latency | P50 | P95 | P99 |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 20 | 20 | 19 | 0 | 1 | 0 | 1 | 2m0.0s | 454.69 tasks/s | 0.16 tasks/s | 1594.4 ms | 1546.4 ms | 2518.7 ms | 2518.7 ms |

### Latency Percentiles

| Run | Observations | Avg | P50 | P95 | P99 |
| --- | ---: | ---: | ---: | ---: | ---: |
| Rep 1 | 19 | 1594.4 ms | 1546.4 ms | 2518.7 ms | 2518.7 ms |

### Task Accounting

```
Accepted(20) = Completed(19) + Failed(0) + Pending(0) + Processing(1) + Unknown(0) [TimedOut=1]  =>  20 = 20  [BALANCED]
```

### Throughput & Submission Rate

| Run | Submission Rate | Completed Throughput |
| --- | ---: | ---: |
| 1 | 454.69 tasks/s | 0.16 tasks/s |

### Interpretation Notes

- Latency measurements include polling interval and HTTP overhead
- Handler simulates execution time; does not represent real workload
- 1 tasks timed out — latency statistics represent only completed tasks

