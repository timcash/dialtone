import type { PolicyDomain } from "./types";

function softmax(values: number[]): number[] {
  const maxVal = values.reduce((m, v) => (v > m ? v : m), -Infinity);
  const expVals = values.map((v) => Math.exp(v - maxVal));
  const total = expVals.reduce((sum, v) => sum + v, 0);
  if (total <= 0) return values.map(() => 0);
  return expVals.map((v) => v / total);
}

export class MarkovChainModel {
  private transition: number[][];
  private baseTransition: number[][];

  constructor(domains: PolicyDomain[]) {
    const count = domains.length;
    this.baseTransition = Array.from({ length: count }, () => Array.from({ length: count }, () => 0));

    for (let source = 0; source < domains.length; source++) {
      const domain = domains[source];
      const outgoing = domain.connections;
      if (outgoing.length === 0) {
        this.baseTransition[source][source] = 1;
        continue;
      }

      const selfWeight = 0.28;
      this.baseTransition[source][source] = selfWeight;
      const rawWeights = domain.connectionWeights;
      const hasCustomWeights = rawWeights && rawWeights.length === outgoing.length;
      const weights = hasCustomWeights ? rawWeights : outgoing.map(() => 1);
      const sumWeights = weights.reduce((s, w) => s + Math.max(0, w), 0);
      const normalizer = sumWeights > 0 ? sumWeights : outgoing.length;
      for (let i = 0; i < outgoing.length; i++) {
        const target = outgoing[i];
        const weight = hasCustomWeights ? Math.max(0, weights[i]) : 1;
        this.baseTransition[source][target] = ((1 - selfWeight) * weight) / normalizer;
      }
    }

    this.transition = this.baseTransition.map((row) => [...row]);
  }

  updateWeights(nodeScores: number[], volatility: number): void {
    const scoreScale = Math.max(0.05, volatility);
    for (let source = 0; source < this.transition.length; source++) {
      const raw: number[] = [];
      const activeTargets: number[] = [];
      for (let target = 0; target < this.transition[source].length; target++) {
        const base = this.baseTransition[source][target];
        if (base <= 0) continue;
        activeTargets.push(target);
        raw.push(Math.log(base + 1e-6) + nodeScores[target] * scoreScale * 1e-7);
      }

      const norm = softmax(raw);
      for (let target = 0; target < this.transition[source].length; target++) {
        this.transition[source][target] = 0;
      }
      for (let i = 0; i < activeTargets.length; i++) {
        this.transition[source][activeTargets[i]] = norm[i];
      }
    }
  }

  getWeight(source: number, target: number): number {
    return this.transition[source]?.[target] ?? 0;
  }

  sampleNext(source: number, rng: () => number): number {
    const row = this.transition[source];
    if (!row) return source;
    let r = rng();
    for (let i = 0; i < row.length; i++) {
      r -= row[i];
      if (r <= 0) return i;
    }
    return source;
  }
}
