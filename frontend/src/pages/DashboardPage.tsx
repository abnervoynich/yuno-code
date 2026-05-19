import { useQuery } from "@tanstack/react-query";
import { listRuns, listTransactions, listBatches } from "../api/client";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
} from "recharts";
import { CheckCircle, AlertTriangle, XCircle, TrendingUp } from "lucide-react";
import { useNavigate } from "react-router-dom";

const COLORS = ["#22c55e", "#f59e0b", "#ef4444", "#8b5cf6", "#f97316"];

export function DashboardPage() {
  const navigate = useNavigate();
  const { data: runs = [] } = useQuery({
    queryKey: ["runs"],
    queryFn: () => listRuns().then((r) => r.data),
  });
  const { data: transactions = [] } = useQuery({
    queryKey: ["transactions"],
    queryFn: () => listTransactions().then((r) => r.data),
  });
  const { data: batches = [] } = useQuery({
    queryKey: ["batches"],
    queryFn: () => listBatches().then((r) => r.data),
  });

  const latestRun = runs[0];

  const pspData = ["pspa", "pspb", "pspc"].map((psp) => ({
    name: psp.toUpperCase(),
    transactions: transactions.filter((t) => t.psp_name === psp).length,
  }));

  const pieData = latestRun
    ? [
        { name: "Matched", value: latestRun.matched_count },
        { name: "Missing", value: latestRun.missing_count },
        { name: "Unexpected", value: latestRun.unexpected_count },
        { name: "Mismatch", value: latestRun.mismatch_count },
        { name: "Fee Error", value: latestRun.fee_error_count },
      ].filter((d) => d.value > 0)
    : [];

  return (
    <div className="p-6">
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-slate-900">Dashboard</h2>
        <p className="text-slate-500 text-sm mt-1">
          LuxeCart Settlement Reconciliation Overview
        </p>
      </div>

      {/* KPI Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        <KPICard
          icon={<TrendingUp className="text-blue-500" size={20} />}
          label="Transactions"
          value={transactions.length.toString()}
          sub="Imported"
          color="blue"
        />
        <KPICard
          icon={<CheckCircle className="text-green-500" size={20} />}
          label="Match Rate"
          value={
            latestRun ? `${latestRun.match_rate.toFixed(1)}%` : "—"
          }
          sub={latestRun ? "Latest run" : "No runs yet"}
          color="green"
        />
        <KPICard
          icon={<AlertTriangle className="text-yellow-500" size={20} />}
          label="Discrepancy"
          value={
            latestRun
              ? `$${Math.abs(latestRun.discrepancy_usd).toFixed(0)}`
              : "—"
          }
          sub="USD difference"
          color="yellow"
        />
        <KPICard
          icon={<XCircle className="text-red-500" size={20} />}
          label="Issues"
          value={
            latestRun
              ? (
                  latestRun.missing_count +
                  latestRun.unexpected_count +
                  latestRun.mismatch_count +
                  latestRun.fee_error_count
                ).toString()
              : "—"
          }
          sub="Needs attention"
          color="red"
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
        {/* PSP Transaction Distribution */}
        <div className="bg-white rounded-xl border border-slate-200 p-5">
          <h3 className="font-semibold text-slate-800 mb-4">
            Transactions by PSP
          </h3>
          <ResponsiveContainer width="100%" height={200}>
            <BarChart data={pspData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
              <XAxis dataKey="name" tick={{ fontSize: 12 }} />
              <YAxis tick={{ fontSize: 12 }} />
              <Tooltip />
              <Bar dataKey="transactions" fill="#3b82f6" radius={4} />
            </BarChart>
          </ResponsiveContainer>
        </div>

        {/* Latest reconciliation pie */}
        <div className="bg-white rounded-xl border border-slate-200 p-5">
          <h3 className="font-semibold text-slate-800 mb-4">
            Latest Reconciliation Result
          </h3>
          {pieData.length > 0 ? (
            <div className="flex items-center gap-4">
              <ResponsiveContainer width="50%" height={200}>
                <PieChart>
                  <Pie
                    data={pieData}
                    cx="50%"
                    cy="50%"
                    innerRadius={50}
                    outerRadius={80}
                    dataKey="value"
                  >
                    {pieData.map((_, i) => (
                      <Cell key={i} fill={COLORS[i % COLORS.length]} />
                    ))}
                  </Pie>
                  <Tooltip />
                </PieChart>
              </ResponsiveContainer>
              <div className="flex-1 space-y-2">
                {pieData.map((d, i) => (
                  <div key={d.name} className="flex items-center gap-2 text-sm">
                    <span
                      className="w-3 h-3 rounded-full shrink-0"
                      style={{ backgroundColor: COLORS[i % COLORS.length] }}
                    />
                    <span className="text-slate-600">{d.name}</span>
                    <span className="ml-auto font-semibold">{d.value}</span>
                  </div>
                ))}
              </div>
            </div>
          ) : (
            <p className="text-slate-400 text-sm text-center py-8">
              No reconciliation runs yet
            </p>
          )}
        </div>
      </div>

      {/* Recent runs */}
      <div className="bg-white rounded-xl border border-slate-200 p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-semibold text-slate-800">Recent Runs</h3>
          <button
            onClick={() => navigate("/reconciliation")}
            className="text-sm text-blue-600 hover:text-blue-700"
          >
            View all →
          </button>
        </div>
        {runs.length === 0 ? (
          <p className="text-slate-400 text-sm py-4 text-center">
            No reconciliation runs yet
          </p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-slate-500 border-b border-slate-100">
                <th className="pb-2 font-medium">Name</th>
                <th className="pb-2 font-medium">Period</th>
                <th className="pb-2 font-medium text-right">Match Rate</th>
                <th className="pb-2 font-medium text-right">Discrepancy</th>
              </tr>
            </thead>
            <tbody>
              {runs.slice(0, 5).map((run) => (
                <tr
                  key={run.id}
                  className="border-b border-slate-50 hover:bg-slate-50 cursor-pointer"
                  onClick={() => navigate(`/reconciliation/${run.id}`)}
                >
                  <td className="py-2.5 font-medium text-blue-600">
                    {run.name || "Unnamed"}
                  </td>
                  <td className="py-2.5 text-slate-500">
                    {run.period_start?.slice(0, 10)} → {run.period_end?.slice(0, 10)}
                  </td>
                  <td className="py-2.5 text-right">
                    <span
                      className={`font-semibold ${run.match_rate >= 90 ? "text-green-600" : run.match_rate >= 70 ? "text-yellow-600" : "text-red-600"}`}
                    >
                      {run.match_rate.toFixed(1)}%
                    </span>
                  </td>
                  <td className="py-2.5 text-right text-slate-700">
                    ${Math.abs(run.discrepancy_usd).toFixed(2)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Settlement batches summary */}
      <div className="mt-4 bg-white rounded-xl border border-slate-200 p-5">
        <h3 className="font-semibold text-slate-800 mb-3">
          Settlement Batches ({batches.length})
        </h3>
        <div className="grid grid-cols-3 gap-3">
          {["pspa", "pspb", "pspc"].map((psp) => {
            const count = batches.filter((b) => b.psp_name === psp).length;
            return (
              <div
                key={psp}
                className="bg-slate-50 rounded-lg p-3 text-center"
              >
                <div className="text-lg font-bold text-slate-800">{count}</div>
                <div className="text-xs text-slate-500 mt-0.5">
                  {psp.toUpperCase()} files
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function KPICard({
  icon,
  label,
  value,
  sub,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  sub: string;
  color: string;
}) {
  return (
    <div className="bg-white rounded-xl border border-slate-200 p-5">
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm font-medium text-slate-500">{label}</span>
        {icon}
      </div>
      <div className="text-2xl font-bold text-slate-900">{value}</div>
      <div className="text-xs text-slate-400 mt-1">{sub}</div>
    </div>
  );
}
