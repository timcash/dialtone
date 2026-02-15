export interface PolicyDomain {
  id: string;
  name: string;
  lat: number;
  lng: number;
  color: number;
  connections: number[];
  connectionWeights?: number[];
}

export type ImpactType = "benefit" | "cost";

export interface ShadowPrice {
  name: string;
  unit: string;
  impactType: ImpactType;
  priceFunction: (rng: () => number, year: number) => number;
}

export interface ImpactFlow {
  shadowPrice: ShadowPrice;
  quantityFunction: (funding: number, year: number) => number;
}

export interface MonteCarloParams {
  years: number;
  iterations: number;
  discountRate: number;
  volatility: number;
}

export interface MonteCarloResult {
  totalNpv: number;
  finalState: number;
  breakdown: Record<string, number>;
}

export interface MonteCarloSummary {
  results: MonteCarloResult[];
  expectedNpv: number;
  successProbability: number;
  histogram: number[];
  xMin: number;
  xMax: number;
  yMin: number;
  yMax: number;
  mean: number;
  meanA: number;
  meanB: number;
  bimodal: boolean;
  breakEvenX: number;
  p10: number;
  p90: number;
}

export interface PolicyPreset {
  id: string;
  label: string;
  scenarioId: string;
  years: number;
  iterations: number;
  discountRate: number;
  volatility: number;
  funding: number[];
}
