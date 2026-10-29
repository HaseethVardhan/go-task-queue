## Experiment: burst

Measure response when batches of tasks arrive quickly.

**Date:** 2026-10-29 16:03:28 IST  
**Duration:** 4m0.0s  

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
| Name | burst |
| Task Count | 20 |
| Submission Concurrency | 5 |
| Polling Concurrency | 10 |
| Poll Interval | 500ms |
| Timeout | 2m0s |
| Failure Probability | 0.10 |
| Repetitions | 1 |
| Batch Sizes | [20 50] |
| Min Duration (ms) | 100 |
| Max Duration (ms) | 500 |

### Sub-Experiments

#### batch-20

| Rep | Submitted | Accepted | Completed | Failed | Timed Out | Pending | Processing | Elapsed | Sub Rate | Throughput | Avg Latency | P50 | P95 | P99 |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 20 | 20 | 17 | 0 | 3 | 0 | 3 | 2m0.0s | 40.83 tasks/s | 0.14 tasks/s | 1647.6 ms | 1768.5 ms | 2613.8 ms | 2613.8 ms |

#### batch-50

| Rep | Submitted | Accepted | Completed | Failed | Timed Out | Pending | Processing | Elapsed | Sub Rate | Throughput | Avg Latency | P50 | P95 | P99 |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 50 | 50 | 46 | 0 | 4 | 0 | 4 | 2m0.0s | 639.08 tasks/s | 0.38 tasks/s | 2817.4 ms | 2566.3 ms | 5086.2 ms | 5089.9 ms |

### Latency Percentiles

| Run | Observations | Avg | P50 | P95 | P99 |
| --- | ---: | ---: | ---: | ---: | ---: |
| batch-20 #1 | 17 | 1647.6 ms | 1768.5 ms | 2613.8 ms | 2613.8 ms |
| batch-50 #1 | 46 | 2817.4 ms | 2566.3 ms | 5086.2 ms | 5089.9 ms |

### Task Accounting

**Run 1:**  
```
Accepted(20) = Completed(17) + Failed(0) + Pending(0) + Processing(3) + Unknown(0) [TimedOut=3]  =>  20 = 20  [BALANCED]
```

**Run 2:**  
```
Accepted(50) = Completed(46) + Failed(0) + Pending(0) + Processing(4) + Unknown(0) [TimedOut=4]  =>  50 = 50  [BALANCED]
```

### Throughput & Submission Rate

| Run | Submission Rate | Completed Throughput |
| --- | ---: | ---: |
| 1 | 40.83 tasks/s | 0.14 tasks/s |
| 2 | 639.08 tasks/s | 0.38 tasks/s |

### Interpretation Notes

- Latency measurements include polling interval and HTTP overhead
- Handler simulates execution time; does not represent real workload
- 7 tasks timed out — latency statistics represent only completed tasks

