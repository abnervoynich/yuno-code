import axios from "axios";
import type {
  Transaction,
  SettlementBatch,
  ReconciliationRun,
  MatchResult,
  ReconciliationSummary,
} from "../types";

const BASE_URL = import.meta.env.VITE_API_URL || "";

const api = axios.create({ baseURL: BASE_URL });

// Transactions
export const importTransactions = (txns: Omit<Transaction, "id">[]) =>
  api.post<{ imported: number }>("/api/v1/transactions/bulk", txns);

export const listTransactions = (params?: {
  psp?: string;
  from?: string;
  to?: string;
}) => api.get<Transaction[]>("/api/v1/transactions/", { params });

// Settlements
export const uploadSettlement = (file: File, pspName: string) => {
  const form = new FormData();
  form.append("file", file);
  form.append("psp_name", pspName);
  return api.post<{
    batch_id: string;
    psp_name: string;
    format: string;
    record_count: number;
    period_start: string;
    period_end: string;
  }>("/api/v1/settlements/upload", form, {
    headers: { "Content-Type": "multipart/form-data" },
  });
};

export const listBatches = () =>
  api.get<SettlementBatch[]>("/api/v1/settlements/batches");

// Reconciliation
export const runReconciliation = (payload: {
  name: string;
  period_start: string;
  period_end: string;
}) => api.post<ReconciliationRun>("/api/v1/reconciliation/run", payload);

export const listRuns = () =>
  api.get<ReconciliationRun[]>("/api/v1/reconciliation/runs");

export const getRunSummary = (id: string) =>
  api.get<ReconciliationSummary>(`/api/v1/reconciliation/runs/${id}/summary`);

export const listResults = (id: string, status?: string) =>
  api.get<MatchResult[]>(`/api/v1/reconciliation/runs/${id}/results`, {
    params: status ? { status } : undefined,
  });
