# Capacity Stress Methodology

1. Run stepped load tests at increasing target arrival rates (RPS).
2. For each step, collect:
   - throughput (http_reqs rate)
   - p95/p99 latency
   - error rate
   - max VUs (proxy for concurrent clients supported)
   - CPU/memory samples from Docker stats
3. Mark a step as unstable when:
   - error rate >= 2% OR
   - p95 > 500ms OR
   - p99 > 1000ms
4. Detect saturation knee where p95 latency increases sharply (>50%) vs previous step.
5. Report:
   - maximum stable QPS
   - maximum concurrent clients (max VUs in stable region)
   - CPU/memory at saturation step
   - throughput before degradation
