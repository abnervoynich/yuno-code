import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { uploadSettlement, listBatches } from "../api/client";
import { Upload, CheckCircle } from "lucide-react";

const PSP_OPTIONS = [
  { value: "pspa", label: "PSP A (CSV, daily)" },
  { value: "pspb", label: "PSP B (JSON, weekly)" },
  { value: "pspc", label: "PSP C (Custom pipe, every 3 days)" },
];

export function SettlementsPage() {
  const qc = useQueryClient();
  const [file, setFile] = useState<File | null>(null);
  const [pspName, setPspName] = useState("pspa");
  const [uploadResult, setUploadResult] = useState<string | null>(null);

  const { data: batches = [], isLoading } = useQuery({
    queryKey: ["batches"],
    queryFn: () => listBatches().then((r) => r.data),
  });

  const uploadMutation = useMutation({
    mutationFn: () => uploadSettlement(file!, pspName),
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: ["batches"] });
      setUploadResult(
        `Uploaded ${res.data.record_count} records from ${res.data.psp_name.toUpperCase()}`
      );
      setFile(null);
    },
    onError: (e: unknown) => {
      const msg =
        (e as { response?: { data?: { error?: string } } })?.response?.data
          ?.error || "Upload failed";
      setUploadResult(`Error: ${msg}`);
    },
  });

  return (
    <div className="p-6">
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-slate-900">Settlement Reports</h2>
        <p className="text-slate-500 text-sm mt-1">
          Upload PSP settlement files for reconciliation
        </p>
      </div>

      {/* Upload form */}
      <div className="bg-white rounded-xl border border-slate-200 p-6 mb-6">
        <h3 className="font-semibold text-slate-800 mb-4">Upload Settlement File</h3>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">
              PSP
            </label>
            <select
              className="w-full border border-slate-300 rounded-lg px-3 py-2 text-sm"
              value={pspName}
              onChange={(e) => setPspName(e.target.value)}
            >
              {PSP_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          </div>
          <div className="md:col-span-2">
            <label className="block text-sm font-medium text-slate-700 mb-1">
              File
            </label>
            <input
              type="file"
              accept=".csv,.json,.txt"
              className="w-full text-sm text-slate-700 file:mr-3 file:py-1.5 file:px-3 file:rounded-lg file:border-0 file:text-sm file:bg-blue-50 file:text-blue-600 hover:file:bg-blue-100"
              onChange={(e) => {
                setFile(e.target.files?.[0] ?? null);
                setUploadResult(null);
              }}
            />
          </div>
        </div>
        <button
          className="flex items-center gap-2 bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
          disabled={!file || uploadMutation.isPending}
          onClick={() => uploadMutation.mutate()}
        >
          <Upload size={15} />
          {uploadMutation.isPending ? "Uploading..." : "Upload"}
        </button>
        {uploadResult && (
          <p
            className={`mt-3 text-sm flex items-center gap-1.5 ${uploadResult.startsWith("Error") ? "text-red-600" : "text-green-700"}`}
          >
            {!uploadResult.startsWith("Error") && <CheckCircle size={14} />}
            {uploadResult}
          </p>
        )}
      </div>

      {/* Batch list */}
      <div className="bg-white rounded-xl border border-slate-200 p-5">
        <h3 className="font-semibold text-slate-800 mb-4">
          Settlement Batches ({batches.length})
        </h3>
        {isLoading ? (
          <p className="text-slate-400 text-sm">Loading...</p>
        ) : batches.length === 0 ? (
          <p className="text-slate-400 text-sm py-6 text-center">
            No batches yet — upload a settlement file above
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-slate-500 border-b border-slate-100">
                  <th className="pb-2 font-medium">PSP</th>
                  <th className="pb-2 font-medium">Format</th>
                  <th className="pb-2 font-medium">Filename</th>
                  <th className="pb-2 font-medium text-right">Records</th>
                  <th className="pb-2 font-medium text-right">Gross</th>
                  <th className="pb-2 font-medium">Period</th>
                  <th className="pb-2 font-medium">Uploaded</th>
                </tr>
              </thead>
              <tbody>
                {batches.map((b) => (
                  <tr
                    key={b.id}
                    className="border-b border-slate-50 hover:bg-slate-50"
                  >
                    <td className="py-2.5 font-medium uppercase">{b.psp_name}</td>
                    <td className="py-2.5 text-slate-500 uppercase">{b.format}</td>
                    <td className="py-2.5 text-slate-600 max-w-xs truncate">
                      {b.filename || "—"}
                    </td>
                    <td className="py-2.5 text-right">{b.record_count}</td>
                    <td className="py-2.5 text-right">
                      {b.total_gross.toFixed(2)} {b.currency}
                    </td>
                    <td className="py-2.5 text-slate-500">
                      {b.period_start?.slice(0, 10)} → {b.period_end?.slice(0, 10)}
                    </td>
                    <td className="py-2.5 text-slate-400">
                      {new Date(b.created_at).toLocaleDateString()}
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
