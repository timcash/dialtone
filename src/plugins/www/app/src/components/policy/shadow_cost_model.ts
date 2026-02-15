import type { ImpactFlow, MonteCarloParams, PolicyDomain, ShadowPrice } from "./types";

function boxMuller(mean: number, std: number, rng: () => number): number {
  const u = Math.max(1e-7, 1 - rng());
  const v = Math.max(1e-7, 1 - rng());
  const z = Math.sqrt(-2 * Math.log(u)) * Math.cos(2 * Math.PI * v);
  return z * std + mean;
}

function clampFunding(funding: number): number {
  return Math.max(0, Math.min(100, funding));
}

type NodeProfile = {
  benefitMult: number;
  costMult: number;
  riskMult: number;
  welfareBias: number;
};

function profileForNode(id: string): NodeProfile {
  if (id.includes("economic-up") || id.includes("stability")) {
    return { benefitMult: 3.2, costMult: 0.42, riskMult: 0.2, welfareBias: 1.6 };
  }
  if (id.includes("backlash") || id.includes("stall") || id.includes("stagnation") || id.includes("dropout")) {
    return { benefitMult: 0.35, costMult: 2.9, riskMult: 3.6, welfareBias: -1.45 };
  }
  if (id.includes("ridership") || id.includes("air-quality") || id.includes("mode-shift")) {
    return { benefitMult: 1.55, costMult: 0.78, riskMult: 0.7, welfareBias: 0.5 };
  }
  return { benefitMult: 1, costMult: 1, riskMult: 1, welfareBias: 0 };
}

export class ShadowCostModel {
  private nodeImpacts: ImpactFlow[][];

  constructor(domains: PolicyDomain[]) {
    const fiscalSpend: ShadowPrice = {
      name: "Fiscal Spend",
      unit: "USD",
      impactType: "cost",
      priceFunction: () => 1,
    };

    const carbonPrice: ShadowPrice = {
      name: "CO2 Benefit",
      unit: "T",
      impactType: "benefit",
      priceFunction: (rng, year) => boxMuller(120 + year * 2.1, 18, rng),
    };

    const resiliencePrice: ShadowPrice = {
      name: "Resilience",
      unit: "Idx",
      impactType: "benefit",
      priceFunction: (rng) => boxMuller(42000, 6000, rng),
    };

    const welfarePrice: ShadowPrice = {
      name: "Welfare",
      unit: "Pts",
      impactType: "benefit",
      priceFunction: (rng) => boxMuller(36000, 5200, rng),
    };

    const riskCost: ShadowPrice = {
      name: "Risk Cost",
      unit: "Idx",
      impactType: "cost",
      priceFunction: (rng) => boxMuller(50000, 7000, rng),
    };

    this.nodeImpacts = domains.map((domain, idx) => {
      const base = 0.6 + (idx % 3) * 0.2;
      const profile = profileForNode(domain.id);
      return [
        {
          shadowPrice: fiscalSpend,
          quantityFunction: (funding) => {
            const f = clampFunding(funding) / 100;
            return (120000 + f * 320000) * base * profile.costMult;
          },
        },
        {
          shadowPrice: carbonPrice,
          quantityFunction: (funding, year) => {
            const f = clampFunding(funding) / 100;
            return (f * 750 + year * 14) * (1.05 + base * 0.2) * profile.benefitMult;
          },
        },
        {
          shadowPrice: resiliencePrice,
          quantityFunction: (funding, year) => {
            const f = clampFunding(funding) / 100;
            return (f * 2.4 + year * 0.08) * (1 + base * 0.1) * profile.benefitMult;
          },
        },
        {
          shadowPrice: welfarePrice,
          quantityFunction: (funding) => {
            const f = clampFunding(funding) / 100;
            return (Math.max(-2.4, f * 3.1 - 0.9) + profile.welfareBias) * (0.8 + base * 0.15) * profile.benefitMult;
          },
        },
        {
          shadowPrice: riskCost,
          quantityFunction: (funding, year) => {
            const f = clampFunding(funding) / 100;
            return Math.max(0, (1.2 - f) * (1 + year * 0.03)) * (0.9 + base * 0.1) * profile.riskMult;
          },
        },
      ];
    });
  }

  estimateExpectedNodeValue(nodeIndex: number, funding: number, params: MonteCarloParams): number {
    const impacts = this.nodeImpacts[nodeIndex] ?? [];
    let total = 0;
    for (let year = 0; year < params.years; year++) {
      const discount = Math.pow(1 + params.discountRate, -year);
      for (const impact of impacts) {
        const qty = impact.quantityFunction(funding, year);
        const expectedPrice = impact.shadowPrice.priceFunction(() => 0.5, year);
        let value = qty * expectedPrice;
        if (impact.shadowPrice.impactType === "cost") {
          value = -Math.abs(value);
        }
        total += value * discount;
      }
    }
    return total;
  }

  evaluateNode(nodeIndex: number, funding: number, year: number, rng: () => number): Record<string, number> {
    const impacts = this.nodeImpacts[nodeIndex] ?? [];
    const breakdown: Record<string, number> = {};

    for (const impact of impacts) {
      const qty = impact.quantityFunction(funding, year);
      const price = impact.shadowPrice.priceFunction(rng, year);
      let value = qty * price;
      if (impact.shadowPrice.impactType === "cost") {
        value = -Math.abs(value);
      }
      breakdown[impact.shadowPrice.name] = (breakdown[impact.shadowPrice.name] ?? 0) + value;
    }

    return breakdown;
  }
}
