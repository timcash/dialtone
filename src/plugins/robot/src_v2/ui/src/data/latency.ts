type AnyRecord = Record<string, any>;

function toMillis(v: any): number | null {
  const n = Number(v);
  if (!Number.isFinite(n) || n <= 0) return null;
  return n < 10_000_000_000 ? n * 1000 : n;
}

function quantile(values: number[], q: number): number {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const i = Math.max(0, Math.min(sorted.length - 1, Math.floor((sorted.length - 1) * q)));
  return sorted[i];
}

export class LatencyEstimator {
  private readonly offsets: number[] = [];
  private readonly maxOffsets = 120;

  estimate(raw: AnyRecord): number | null {
    const eventTs = toMillis(raw.t_raw ?? raw.timestamp);
    if (eventTs == null) return null;
    const pubTs = toMillis(raw.t_pub ?? raw.timestamp ?? raw.t_raw) ?? eventTs;

    const now = Date.now();
    const offsetSample = now - pubTs;
    if (Number.isFinite(offsetSample)) {
      this.offsets.push(offsetSample);
      if (this.offsets.length > this.maxOffsets) this.offsets.shift();
    }

    // 10th percentile approximates minimum path delay + clock skew.
    const skew = quantile(this.offsets, 0.1);
    let adjusted = now - eventTs - skew;
    if (!Number.isFinite(adjusted)) return null;
    if (adjusted < 0) adjusted = 0;
    if (adjusted > 60_000) return null;
    return adjusted;
  }
}

