import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { listTransactions, importTransactions } from "../api/client";
import { Upload } from "lucide-react";

export function TransactionsPage() {
  const qc = useQueryClient();
  const [pspFilter, setPspFilter] = useState("");
  const [fileError, setFileError] = useState<string | null>(null);
  const [importMsg, setImportMsg] = useState<string | null>(null);

  const { data: transactions = [], isLoading } = useQuery({
    queryKey: ["transactions", pspFilter],
    queryFn: () =>
      listTransactions(pspFilter ? { psp: pspFilter } : undefined).then(
        (r) => r.data
      ),
  });

  const importMutation = useMutation({
    mutationFn: importTransactions,
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: ["transactions"] });
      setImportMsg(`Imported ${res.data.imported} transactions`);
    },
    onError: () => setImportMsg("Import failed — check JSON format"),
  });

  const handleFileImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setFileError(null);
    setImportMsg(null);
    try {
      const text = await file.text();
      const data = JSON.parse(text);
      importMutation.mutate(Array.isArray(data) ? data : [data]);
    } catch {
      setFileError("Invalid JSON file");
    }
    e.target.value = "";
  };

  const total = transactions.length;
  const sales = transactions.filter((t) => t.type === "sale").length;
  const refunds = transactions.filter((t) => t.type === "refund").length;

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-2xl font-bold text-slate-900">Transactions</h2>
          <p className="text-slate-500 text-sm mt-1">
            {total} transactions — {sales} sales, {refunds} refunds
          </p>
        </div>
        <label className="flex items-center gap-2 bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 cursor-pointer">
          <Upload size={15} />
          Import JSON
          <input
            type="file"
            accept=".json"
            className="hidden"
            onChange={handleFileImport}
          />
        </label>
      </div>

      {(fileError || importMsg) && (
        <div
          className={`mb-4 text-sm rounded-lg px-4 py-3 ${fileError ? "bg-red-50 text-red-700" : "bg-green-50 text-green-700"}`}
        >
          {fileError || importMsg}
        </div>
      )}

      {/* Filters */}
      <div className="bg-white rounded-xl border border-slate-200 p-4 mb-4">
        <div className="flex gap-3 items-center">
          <label className="text-sm font-medium text-slate-600">
            Filter by PSP:
          </label>
          {["", "pspa", "pspb", "pspc"].map((p) => (
            <button
              key={p}
              onClick={() => setPspFilter(p)}
              className={`px-3 py-1 rounded-full text-sm font-medium transition-colors ${pspFilter === p ? "bg-blue-600 text-white" : "bg-slate-100 text-slate-600 hover:bg-slate-200"}`}
            >
              {p === "" ? "All" : p.toUpperCase()}
            </button>
          ))}
        </div>
      </div>

      {/* Table */}
      <div className="bg-white rounded-xl border border-slate-200 p-5">
        {isLoading ? (
          <p className="text-slate-400 text-sm text-center py-8">Loading...</p>
        ) : transactions.length === 0 ? (
          <p className="text-slate-400 text-sm text-center py-8">
            No transactions found — import the expected_transactions.json file
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-slate-500 border-b border-slate-100">
                  <th className="pb-2 font-medium">External ID</th>
                  <th className="pb-2 font-medium">Type</th>
                  <th className="pb-2 font-medium">PSP</th>
                  <th className="pb-2 font-medium text-right">Amount</th>
                  <th className="pb-2 font-medium">Created</th>
                  <th className="pb-2 font-medium">Expected Settlement</th>
                </tr>
              </thead>
              <tbody>
                {transactions.map((t) => (
                  <tr
                    key={t.id}
                    className="border-b border-slate-50 hover:bg-slate-50"
                  >
                    <td className="py-2.5 font-mono text-xs text-slate-700">
                      {t.external_id}
                    </td>
                    <td className="py-2.5">
                      <span
                        className={`inline-flex px-2 py-0.5 rounded text-xs font-medium ${t.type === "refund" ? "bg-red-100 text-red-700" : "bg-blue-100 text-blue-700"}`}
                      >
                        {t.type}
                      </span>
                    </td>
                    <td className="py-2.5 uppercase font-medium text-slate-600">
                      {t.psp_name}
                    </td>
                    <td className="py-2.5 text-right font-semibold">
                      <span
                        className={
                          t.amount < 0 ? "text-red-600" : "text-slate-900"
                        }
                      >
                        {t.amount < 0 ? "-" : ""}
                        {Math.abs(t.amount).toFixed(2)} {t.currency}
                      </span>
                    </td>
                    <td className="py-2.5 text-slate-500">
                      {new Date(t.created_at).toLocaleString()}
                    </td>
                    <td className="py-2.5 text-slate-500">
                      {t.expected_settle_date?.slice(0, 10)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
