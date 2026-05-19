import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { getRunSummary, listResults } from "../api/client";
import { StatusBadge } from "../components/StatusBadge";
import { ArrowLeft, AlertTriangle } from "lucide-react";
import type { MatchResult } from "../types";

const TABS = [
  { key: "summary", label: "Summary" },
  { key: "missing_from_settlement", label: "Missing" },
  { key: "unexpected_in_settlement", label: "Unexpected" },
  { key: "amount_mismatch", label: "Amt Mismatch" },
  { key: "fee_mismatch", label: "Fee Errors" },
  { key: "matched", label: "Matched" },
] as const;

type Tab = (typeof TABS)[number]["key"];

export function RunDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [tab, setTab] = useState<Tab>("summary");

  const { data: summary, isLoading: summaryLoading } = useQuery({
    queryKey: ["summary", id],
    queryFn: () => getRunSummary(id!).then((r) => r.data),
    enabled: !!id,
  });

  const { data: results = [] } = useQuery({
    queryKey: ["results", id, tab],
    queryFn: () =>
      listResults(id!, tab === "summary" ? undefined : tab).then((r) => r.data),
    enabled: !!id && tab !== "summary",
  });

  if (summaryLoading) {
    return (
      <div className="p-6 text-slate-400 text-sm">Loading...</div>
    );
  }
  if (!summary) {
    return (
      <div className="p-6 text-red-600 text-sm">Run not found</div>
    );
  }

  const { run } = summary;

  return (
    <div className="p-6">
      {/* Header */}
      <div className="flex items-center gap-3 mb-6">
        <button
          onClick={() => navigate("/reconciliation")}
          className="text-slate-400 hover:text-slate-600"
        >
          <ArrowLeft size={20} />
        </button>
        <div>
          <h2 className="text-2xl font-bold text-slate-900">
            {run.name || "Reconciliation Run"}
          </h2>
          <p className="text-sm text-slate-400 mt-0.5">
            {run.period_start?.slice(0, 10)} → {run.period_end?.slice(0, 10)}
            {" · "}
            <StatusBadge status={run.status} small />
          </p>
        </div>
      </div>

      {/* Stats row */}
      <div className="grid grid-cols-2 md:grid-cols-6 gap-3 mb-6">
        {[
          { label: "Match Rate", value: `${run.match_rate.toFixed(1)}%`, color: "text-green-700" },
          { label: "Matched", value: run.matched_count, color: "text-slate-900" },
          { label: "Missing", value: run.missing_count, color: "text-red-600" },
          { label: "Unexpected", value: run.unexpected_count, color: "text-orange-600" },
          { label: "Mismatch", value: run.mismatch_count, color: "text-yellow-700" },
          { label: "Fee Errors", value: run.fee_error_count, color: "text-purple-700" },
        ].map((s) => (
          <div
            key={s.label}
            className="bg-white rounded-xl border border-slate-200 p-3 text-center"
          >
            <div className={`text-xl font-bold ${s.color}`}>{s.value}</div>
            <div className="text-xs text-slate-400 mt-0.5">{s.label}</div>
          </div>
        ))}
      </div>

      {/* Discrepancy alert */}
      {run.discrepancy_usd !== 0 && (
        <div className="bg-amber-50 border border-amber-200 rounded-xl p-4 mb-6 flex items-center gap-3">
          <AlertTriangle size={18} className="text-amber-600 shrink-0" />
          <div>
            <span className="font-semibold text-amber-800">
              ${Math.abs(run.discrepancy_usd).toFixed(2)} USD discrepancy
            </span>
            <span className="text-amber-600 text-sm ml-2">
              between expected (${run.total_expected.toFixed(2)}) and settled (${run.total_settled.toFixed(2)})
            </span>
          </div>
        </div>
      )}

      {/* Tabs */}
      <div className="border-b border-slate-200 mb-6">
        <div className="flex gap-1">
          {TABS.map((t) => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={`px-4 py-2 text-sm font-medium rounded-t-lg border-b-2 transition-colors ${
                tab === t.key
                  ? "border-blue-600 text-blue-700"
                  : "border-transparent text-slate-500 hover:text-slate-700"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {tab === "summary" ? (
        <SummaryTab summary={summary} />
      ) : (
        <ResultsTable results={results} />
      )}
    </div>
  );
}

function SummaryTab({
  summary,
}: {
  summary: NonNullable<ReturnType<typeof useQuery>["data"]>;
}) {
  const s = summary as Awaited<ReturnType<typeof getRunSummary>>["data"];
  return (
    <div className="space-y-6">
      {/* PSP Breakdowns */}
      <div className="bg-white rounded-xl border border-slate-200 p-5">
        <h3 className="font-semibold text-slate-800 mb-4">PSP Breakdown</h3>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-slate-500 border-b border-slate-100">
                <th className="pb-2 font-medium">PSP</th>
                <th className="pb-2 font-medium text-right">Expected</th>
                <th className="pb-2 font-medium text-right">Settled</th>
                <th className="pb-2 font-medium text-right">Matched</th>
                <th className="pb-2 font-medium text-right">Missing</th>
                <th className="pb-2 font-medium text-right">Unexpected</th>
                <th className="pb-2 font-medium text-right">Match %</th>
              </tr>
            </thead>
            <tbody>
              {(s.psp_breakdowns || []).map((b) => (
                <tr key={b.psp_name} className="border-b border-slate-50">
                  <td className="py-2.5 font-medium uppercase">{b.psp_name}</td>
                  <td className="py-2.5 text-right">${b.total_expected.toFixed(2)}</td>
                  <td className="py-2.5 text-right">${b.total_settled.toFixed(2)}</td>
                  <td className="py-2.5 text-right text-green-700">{b.matched_count}</td>
                  <td className="py-2.5 text-right text-red-600">{b.missing_count}</td>
                  <td className="py-2.5 text-right text-orange-600">{b.unexpected_count}</td>
                  <td className="py-2.5 text-right">
                    <span
                      className={`font-semibold ${b.match_rate >= 90 ? "text-green-700" : b.match_rate >= 70 ? "text-yellow-700" : "text-red-600"}`}
                    >
                      {b.match_rate.toFixed(1)}%
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Top discrepancies */}
      {(s.top_discrepancies || []).length > 0 && (
        <div className="bg-white rounded-xl border border-slate-200 p-5">
          <h3 className="font-semibold text-slate-800 mb-4">
            Top Discrepancies (by $ amount)
          </h3>
          <ResultsTable results={s.top_discrepancies} />
        </div>
      )}
    </div>
  );
}

function ResultsTable({ results }: { results: MatchResult[] }) {
  if (!results || results.length === 0) {
    return (
      <div className="bg-white rounded-xl border border-slate-200 p-8 text-center text-slate-400 text-sm">
        No records in this category
      </div>
    );
  }

  return (
    <div className="bg-white rounded-xl border border-slate-200 p-5">
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-slate-500 border-b border-slate-100">
              <th className="pb-2 font-medium">Status</th>
              <th className="pb-2 font-medium">PSP</th>
              <th className="pb-2 font-medium text-right">Expected ($)</th>
              <th className="pb-2 font-medium text-right">Actual ($)</th>
              <th className="pb-2 font-medium text-right">Diff ($)</th>
              <th className="pb-2 font-medium text-right">Confidence</th>
              <th className="pb-2 font-medium">Notes</th>
            </tr>
          </thead>
          <tbody>
            {results.map((r) => (
              <tr key={r.id} className="border-b border-slate-50 hover:bg-slate-50">
                <td className="py-2.5">
                  <StatusBadge status={r.status} small />
                </td>
                <td className="py-2.5 uppercase font-medium">{r.psp_name}</td>
                <td className="py-2.5 text-right">
                  {r.expected_amount > 0 ? r.expected_amount.toFixed(2) : "—"}
                </td>
                <td className="py-2.5 text-right">
                  {r.actual_amount > 0 ? r.actual_amount.toFixed(2) : "—"}
                </td>
                <td className="py-2.5 text-right">
                  {r.amount_diff_usd !== 0 ? (
                    <span
                      className={r.amount_diff_usd < 0 ? "text-red-600" : "text-green-700"}
                    >
                      {r.amount_diff_usd > 0 ? "+" : ""}
                      {r.amount_diff_usd.toFixed(2)}
                    </span>
                  ) : (
                    <span className="text-slate-400">0.00</span>
                  )}
                </td>
                <td className="py-2.5 text-right text-slate-500">
                  {(r.confidence_score * 100).toFixed(0)}%
                </td>
                <td className="py-2.5 text-slate-500 max-w-xs truncate">
                  {r.notes || "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
