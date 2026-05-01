# Dataloader Impact Benchmark

- Avg SQL queries/request: 1.01 -> 1.00 (0.83% reduction)
- productCategory resolver avg latency: 17.693 ms -> 17.618 ms (0.42% reduction)
- Throughput: 14.63 rps -> 14.51 rps (-0.82% improvement)
- Category GetByID DB calls: 0 -> 0 (0.00% reduction)

## Resume-ready bullets

- Reduced SQL queries per GraphQL request by 0.83% by enabling dataloader batching for category resolution.
- Improved GraphQL resolver performance by 0.42% on productCategory and increased throughput by -0.82%.
