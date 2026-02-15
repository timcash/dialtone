import { MarkovChainModel } from "./markov_chain_model";
import { MonteCarloModel } from "./monte_carlo_model";
import { ShadowCostModel } from "./shadow_cost_model";
import type { MonteCarloParams, PolicyDomain } from "./types";

export function runPolicyModelTests(domains: PolicyDomain[]): string[] {
  const issues: string[] = [];

  const markov = new MarkovChainModel(domains);
  const shadow = new ShadowCostModel(domains);
  const monteCarlo = new MonteCarloModel(markov, shadow);

  const params: MonteCarloParams = {
    years: 6,
    iterations: 80,
    discountRate: 0.04,
    volatility: 0.5,
  };

  const scores = domains.map((_, idx) => shadow.estimateExpectedNodeValue(idx, 55, params));
  markov.updateWeights(scores, params.volatility);

  for (let source = 0; source < domains.length; source++) {
    let sum = 0;
    for (let target = 0; target < domains.length; target++) {
      const w = markov.getWeight(source, target);
      if (w < -1e-6 || w > 1 + 1e-6) {
        issues.push(`invalid markov weight s${source}->t${target}: ${w}`);
      }
      sum += w;
    }
    if (Math.abs(sum - 1) > 1e-4) {
      issues.push(`markov row does not normalize for source ${source}: ${sum}`);
    }
  }

  const sampleBreakdown = shadow.evaluateNode(0, 50, 0, Math.random);
  if (Object.keys(sampleBreakdown).length === 0) {
    issues.push("shadow cost model produced empty breakdown");
  }

  const funding = domains.map(() => 55);
  const sim = monteCarlo.run(0, funding, params);
  if (sim.results.length !== params.iterations) {
    issues.push(`monte carlo iterations mismatch: ${sim.results.length}`);
  }
  if (sim.histogram.length === 0) {
    issues.push("monte carlo histogram was empty");
  }

  return issues;
}
