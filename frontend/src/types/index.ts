export interface Transaction {
  id: string;
  external_id: string;
  amount: number;
  currency: string;
  status: string;
  type: string;
  psp_name: string;
  customer_ref: string;
  created_at: string;
  expected_settle_date: string;
}

export interface SettlementBatch {
  id: string;
  psp_name: string;
  format: string;
  filename: string;
  period_start: string;
  period_end: string;
  total_gross: number;
  total_fees: number;
  total_net: number;
  currency: string;
  record_count: number;
  status: string;
  created_at: string;
}

export interface ReconciliationRun {
  id: string;
  name: string;
  period_start: string;
  period_end: string;
  status: string;
  total_expected: number;
  total_settled: number;
  matched_count: number;
  missing_count: number;
  unexpected_count: number;
  mismatch_count: number;
  fee_error_count: number;
  match_rate: number;
  discrepancy_usd: number;
  created_at: string;
  completed_at?: string;
}

export interface MatchResult {
  id: string;
  reconciliation_run_id: string;
  transaction_id?: string;
  settlement_record_id?: string;
  status: MatchStatus;
  confidence_score: number;
  expected_amount: number;
  actual_amount: number;
  amount_diff_usd: number;
  expected_fee: number;
  actual_fee: number;
  currency: string;
  psp_name: string;
  notes?: string;
  created_at: string;
}

export type MatchStatus =
  | "matched"
  | "fuzzy_match"
  | "missing_from_settlement"
  | "unexpected_in_settlement"
  | "amount_mismatch"
  | "fee_mismatch";

export interface PSPBreakdown {
  psp_name: string;
  total_expected: number;
  total_settled: number;
  matched_count: number;
  missing_count: number;
  unexpected_count: number;
  mismatch_count: number;
  match_rate: number;
}

export interface ReconciliationSummary {
  run: ReconciliationRun;
  psp_breakdowns: PSPBreakdown[];
  top_discrepancies: MatchResult[];
  missing_items: MatchResult[];
  unexpected_items: MatchResult[];
  amount_mismatches: MatchResult[];
  fee_errors: MatchResult[];
}
