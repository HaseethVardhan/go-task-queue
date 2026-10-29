## Experiment: workload-profiles

Evaluate GoTask throughput across varying task mix profiles.

**Date:** 2026-10-29 16:07:28 IST  
**Duration:** 6m0.0s  

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
| Name | workload_profile |
| Task Count | 30 |
| Submission Concurrency | 5 |
| Polling Concurrency | 10 |
| Poll Interval | 500ms |
| Timeout | 2m0s |
| Failure Probability | 0.10 |
| Repetitions | 1 |
| Profile: email_heavy | email=70% report=20% image=10% |
| Profile: report_heavy | email=20% report=60% image=20% |
| Profile: balanced | email=34% report=33% image=33% |

### Sub-Experiments

#### email_heavy

| Rep | Submitted | Accepted | Completed | Failed | Timed Out | Pending | Processing | Elapsed | Sub Rate | Throughput | Avg Latency | P50 | P95 | P99 |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 30 | 30 | 26 | 0 | 4 | 0 | 4 | 2m0.0s | 523.21 tasks/s | 0.22 tasks/s | 2163.7 ms | 2108.6 ms | 3518.6 ms | 3591.9 ms |

#### report_heavy

| Rep | Submitted | Accepted | Completed | Failed | Timed Out | Pending | Processing | Elapsed | Sub Rate | Throughput | Avg Latency | P50 | P95 | P99 |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 30 | 30 | 27 | 0 | 3 | 0 | 3 | 2m0.0s | 573.47 tasks/s | 0.22 tasks/s | 2197.0 ms | 2101.0 ms | 3556.6 ms | 4019.5 ms |

#### balanced

| Rep | Submitted | Accepted | Completed | Failed | Timed Out | Pending | Processing | Elapsed | Sub Rate | Throughput | Avg Latency | P50 | P95 | P99 |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 30 | 30 | 26 | 0 | 4 | 0 | 4 | 2m0.0s | 584.17 tasks/s | 0.22 tasks/s | 2101.1 ms | 2040.2 ms | 3547.2 ms | 4019.7 ms |

### Latency Percentiles

| Run | Observations | Avg | P50 | P95 | P99 |
| --- | ---: | ---: | ---: | ---: | ---: |
| email_heavy #1 | 26 | 2163.7 ms | 2108.6 ms | 3518.6 ms | 3591.9 ms |
| report_heavy #1 | 27 | 2197.0 ms | 2101.0 ms | 3556.6 ms | 4019.5 ms |
| balanced #1 | 26 | 2101.1 ms | 2040.2 ms | 3547.2 ms | 4019.7 ms |

### Task Accounting

**Run 1:**  
```
Accepted(30) = Completed(26) + Failed(0) + Pending(0) + Processing(4) + Unknown(0) [TimedOut=4]  =>  30 = 30  [BALANCED]
```

**Run 2:**  
```
Accepted(30) = Completed(27) + Failed(0) + Pending(0) + Processing(3) + Unknown(0) [TimedOut=3]  =>  30 = 30  [BALANCED]
```

**Run 3:**  
```
Accepted(30) = Completed(26) + Failed(0) + Pending(0) + Processing(4) + Unknown(0) [TimedOut=4]  =>  30 = 30  [BALANCED]
```

### Throughput & Submission Rate

| Run | Submission Rate | Completed Throughput |
| --- | ---: | ---: |
| 1 | 523.21 tasks/s | 0.22 tasks/s |
| 2 | 573.47 tasks/s | 0.22 tasks/s |
| 3 | 584.17 tasks/s | 0.22 tasks/s |

### Interpretation Notes

- Latency measurements include polling interval and HTTP overhead
- Handler simulates execution time; does not represent real workload
- 11 tasks timed out — latency statistics represent only completed tasks

