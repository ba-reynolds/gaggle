-- Background-sampled host metrics so the admin dashboard can show how the
-- box performed an hour / day / week ago (not just right now).
--   * one row per sample (default: every 60s), written by the metrics Sampler
--   * columns mirror internal/metrics.HostStats
--   * pruned after METRICS_RETENTION_DAYS (default 90) by the sampler loop
--
-- Charts read this table via date_bin() downsampling so long ranges don't
-- ship thousands of points to the browser.

CREATE TABLE host_metrics_samples (
    id             BIGSERIAL PRIMARY KEY,
    cpu_percent    DOUBLE PRECISION NOT NULL,
    mem_total      BIGINT NOT NULL,
    mem_used       BIGINT NOT NULL,
    mem_percent    DOUBLE PRECISION NOT NULL,
    load1          DOUBLE PRECISION NOT NULL,
    load5          DOUBLE PRECISION NOT NULL,
    load15         DOUBLE PRECISION NOT NULL,
    uptime_seconds DOUBLE PRECISION NOT NULL,
    disk_total     BIGINT NOT NULL,
    disk_used      BIGINT NOT NULL,
    disk_percent   DOUBLE PRECISION NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX host_metrics_samples_created_at_idx ON host_metrics_samples (created_at DESC);