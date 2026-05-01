# Cache Effectiveness Benchmark

- Cache hit ratio: 50.00%
- Avg latency with cache hit: 0.294 ms
- Avg latency with cache miss: 0.311 ms
- Latency improvement from caching: 5.50%
- Database offload due to caching: 50.00%

## Resume-ready bullets

- Improved product read path efficiency with Redis, achieving a 50.00% cache hit ratio under benchmark load.
- Reduced read latency by 5.50% (avg hit: 0.294 ms vs miss: 0.311 ms).
- Offloaded approximately 50.00% of product-by-id lookups from PostgreSQL through effective cache utilization.

## Formula references

- Hit rate % = cache_hits / (cache_hits + cache_misses) * 100
- Latency reduction % = (avg_miss_latency_ms - avg_hit_latency_ms) / avg_miss_latency_ms * 100
- DB offload % = cache_hits / (cache_hits + cache_misses) * 100
