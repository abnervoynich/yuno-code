import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { listRuns, runReconciliation } from "../api/client";
import { useNavigate } from "react-router-dom";
import { Play, ChevronRight } from "lucide-react";
import { StatusBadge } from "../components/StatusBadge";

export function ReconciliationPage() {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [name, setName] = useState("Dec 2024 Full Week");
  const [periodStart, setPeriodStart] = useState("2024-12-01");
  const [periodEnd, setPeriodEnd] = useState("2024-12-07");
  const [runError, setRunError] = useState<string | null>(null);

  const { data: runs = [], isLoading } = useQuery({
    queryKey: ["runs"],
    queryFn: () => listRuns().then((r) => r.data),
  });

  const runMutation = useMutation({
    mutationFn: () =>
      runReconciliation({ name, period_start: periodStart, period_end: periodEnd }),
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: ["runs"] });
      navigate(`/reconciliation/${res.data.id}`);
    },
    onError: (e: unknown) => {
      const msg =
        (e as { response?: { data?: { error?: string } } })?.response?.data
          ?.error || "Reconciliation failed";
      setRunError(msg);
    },
  });

  return (
    <div className="p-6">
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-slate-900">Reconciliation</h2>
        <p className="text-slate-500 text-sm mt-1">
          Run reconciliation over a date period to match transactions against
          settlement records
        </p>
      </div>

      {/* Run form */}
      <div className="bg-white rounded-xl border border-slate-200 p-6 mb-6">
        <h3 className="font-semibold text-slate-800 mb-4">New Reconciliation Run</h3>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">
              Run Name
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full border border-slate-300 rounded-lg px-3 py-2 text-sm"
              placeholder="e.g. December 2024"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">
              Period Start
            </label>
            <input
              type="date"
              value={periodStart}
              onChange={(e) => setPeriodStart(e.target.value)}
              className="w-full border border-slate-300 rounded-lg px-3 py-2 text-sm"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">
              Period End
            </label>
            <input
              type="date"
              value={periodEnd}
              onChange={(e) => setPeriodEnd(e.target.value)}
              className="w-full border border-slate-300 rounded-lg px-3 py-2 text-sm"
            />
          </div>
        </div>
        <button
          className="flex items-center gap-2 bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
          disabled={runMutation.isPending}
          onClick={() => {
            setRunError(null);
            runMutation.mutate();
          }}
        >
          <Play size={15} />
          {runMutation.isPending ? "Running..." : "Run Reconciliation"}
        </button>
        {runError && (
          <p className="mt-3 text-sm text-red-600">{runError}</p>
        )}
      </div>

      {/* Runs list */}
      <div className="bg-white rounded-xl border border-slate-200 p-5">
        <h3 className="font-semibold text-slate-800 mb-4">
          Reconciliation Runs ({runs.length})
        </h3>
        {isLoading ? (
          <p className="text-slate-400 text-sm">Loading...</p>
        ) : runs.length === 0 ? (
          <p className="text-slate-400 text-sm text-center py-6">
            No runs yet — click "Run Reconciliation" to start
          </p>
        ) : (
          <div className="space-y-2">
            {runs.map((run) => (
              <div
                key={run.id}
                className="flex items-center gap-4 p-4 rounded-lg border border-slate-100 hover:bg-slate-50 cursor-pointer group"
                onClick={() => navigate(`/reconciliation/${run.id}`)}
              >
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="font-medium text-slate-900">
                      {run.name || "Unnamed Run"}
                    </span>
                    <StatusBadge status={run.status} small />
                  </div>
                  <span className="text-xs text-slate-400">
                    {run.period_start?.slice(0, 10)} → {run.period_end?.slice(0, 10)}
                    {" · "}
                    {new Date(run.created_at).toLocaleDateString()}
                  </span>
                </div>
                <div className="grid grid-cols-4 gap-4 text-sm text-right">
                  <div>
                    <div className="font-semibold text-green-600">
                      {run.match_rate.toFixed(1)}%
                    </div>
                    <div className="text-xs text-slate-400">match rate</div>
                  </div>
                  <div>
                    <div className="font-semibold">{run.matched_count}</div>
                    <div className="text-xs text-slate-400">matched</div>
                  </div>
                  <div>
                    <div className="font-semibold text-red-600">
                      {run.missing_count}
                    </div>
                    <div className="text-xs text-slate-400">missing</div>
                  </div>
                  <div>
                    <div className="font-semibold text-yellow-600">
                      {run.mismatch_count + run.fee_error_count}
                    </div>
                    <div className="text-xs text-slate-400">issues</div>
                  </div>
                </div>
                <ChevronRight
                  size={16}
                  className="text-slate-300 group-hover:text-slate-500 shrink-0"
                />
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
