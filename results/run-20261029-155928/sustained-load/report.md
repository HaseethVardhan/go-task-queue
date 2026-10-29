## Experiment: sustained-load

Evaluate performance under controlled, continuous task arrival rates.

**Date:** 2026-10-29 16:13:28 IST  
**Duration:** 3m0.0s  

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
| Name | sustained_rate |
| Task Count | 50 |
| Submission Concurrency | 5 |
| Polling Concurrency | 10 |
| Poll Interval | 500ms |
| Timeout | 1m0s |
| Failure Probability | 0.10 |
| Repetitions | 1 |
| Rates (tasks/s) | [5 10 20] |
| Rate Duration | 15s |
| Min Duration (ms) | 100 |
| Max Duration (ms) | 500 |

### Sub-Experiments

#### rate-5.0

| Rep | Submitted | Accepted | Completed | Failed | Timed Out | Pending | Processing | Elapsed | Sub Rate | Throughput | Avg Latency | P50 | P95 | P99 |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 75 | 75 | 55 | 0 | 20 | 0 | 10 | 1m0.0s | 5.00 tasks/s | 0.92 tasks/s | 13051.9 ms | 12891.4 ms | 15013.2 ms | 15314.4 ms |

#### rate-10.0

| Rep | Submitted | Accepted | Completed | Failed | Timed Out | Pending | Processing | Elapsed | Sub Rate | Throughput | Avg Latency | P50 | P95 | P99 |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 150 | 150 | 74 | 0 | 76 | 0 | 10 | 1m0.0s | 9.99 tasks/s | 1.23 tasks/s | 15135.2 ms | 14723.5 ms | 18058.7 ms | 19263.3 ms |

#### rate-20.0

| Rep | Submitted | Accepted | Completed | Failed | Timed Out | Pending | Processing | Elapsed | Sub Rate | Throughput | Avg Latency | P50 | P95 | P99 |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 299 | 299 | 118 | 0 | 181 | 0 | 10 | 1m0.0s | 19.93 tasks/s | 1.97 tasks/s | 17619.6 ms | 16699.0 ms | 23468.7 ms | 25224.0 ms |

### Latency Percentiles

| Run | Observations | Avg | P50 | P95 | P99 |
| --- | ---: | ---: | ---: | ---: | ---: |
| rate-5.0 #1 | 55 | 13051.9 ms | 12891.4 ms | 15013.2 ms | 15314.4 ms |
| rate-10.0 #1 | 74 | 15135.2 ms | 14723.5 ms | 18058.7 ms | 19263.3 ms |
| rate-20.0 #1 | 118 | 17619.6 ms | 16699.0 ms | 23468.7 ms | 25224.0 ms |

### Task Accounting

**Run 1:**  
```
Accepted(75) = Completed(55) + Failed(0) + Pending(0) + Processing(10) + Unknown(10) [TimedOut=20]  =>  75 = 75  [BALANCED]
```

**Run 2:**  
```
Accepted(150) = Completed(74) + Failed(0) + Pending(0) + Processing(10) + Unknown(66) [TimedOut=76]  =>  150 = 150  [BALANCED]
```

**Run 3:**  
```
Accepted(299) = Completed(118) + Failed(0) + Pending(0) + Processing(10) + Unknown(171) [TimedOut=181]  =>  299 = 299  [BALANCED]
```

### Throughput & Submission Rate

| Run | Submission Rate | Completed Throughput |
| --- | ---: | ---: |
| 1 | 5.00 tasks/s | 0.92 tasks/s |
| 2 | 9.99 tasks/s | 1.23 tasks/s |
| 3 | 19.93 tasks/s | 1.97 tasks/s |

### Interpretation Notes

- Latency measurements include polling interval and HTTP overhead
- Handler simulates execution time; does not represent real workload
- 277 tasks timed out — latency statistics represent only completed tasks

