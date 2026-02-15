import { MarkovChainModel } from "./markov_chain_model";
import { ShadowCostModel } from "./shadow_cost_model";
import type { MonteCarloParams, MonteCarloResult, MonteCarloSummary } from "./types";

function clamp01(v: number): number {
  return Math.max(0, Math.min(1, v));
}

export class MonteCarloModel {
  constructor(
    private markov: MarkovChainModel,
    private shadowCost: ShadowCostModel,
  ) {}

  run(startNode: number, funding: number[], params: MonteCarloParams): MonteCarloSummary {
    const results: MonteCarloResult[] = [];
    let total = 0;
    let success = 0;

    for (let iter = 0; iter < params.iterations; iter++) {
      let current = startNode;
      let npv = 0;
      const breakdown: Record<string, number> = {};

      for (let year = 0; year < params.years; year++) {
        const discountFactor = Math.pow(1 + params.discountRate, -year);
        const noiseScale = 1 + (Math.random() - 0.5) * params.volatility * 0.3;
        const impacts = this.shadowCost.evaluateNode(current, funding[current] ?? 50, year, Math.random);

        for (const [key, rawValue] of Object.entries(impacts)) {
          const value = rawValue * noiseScale;
          const discounted = value * discountFactor;
          breakdown[key] = (breakdown[key] ?? 0) + discounted;
          npv += discounted;
        }

        current = this.markov.sampleNext(current, Math.random);
      }

      if (npv >= 0) success++;
      total += npv;
      results.push({ totalNpv: npv, finalState: current, breakdown });
    }

    const expected = results.length > 0 ? total / results.length : 0;
    const successProbability = results.length > 0 ? success / results.length : 0;

    const values = results.map((r) => r.totalNpv);
    const min = Math.min(...values, 0);
    const max = Math.max(...values, 1);
    const span = Math.max(1, max - min);
    const binCount = 28;
    const bins = Array.from({ length: binCount }, () => 0);

    for (const value of values) {
      const idx = Math.min(binCount - 1, Math.floor(((value - min) / span) * binCount));
      bins[idx]++;
    }

    const peak = Math.max(...bins, 1);
    const histogram = bins.map((v) => clamp01(v / peak));

    return {
      results,
      expectedNpv: expected,
      successProbability,
      histogram,
      xMin: min,
      xMax: max,
      yMin: 0,
      yMax: peak,
      mean: expected,
      meanA: expected,
      meanB: expected,
      bimodal: false,
      breakEvenX: 0,
      p10: min + (max - min) * 0.1,
      p90: min + (max - min) * 0.9,
    };
  }
}
