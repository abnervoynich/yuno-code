import type { MatchStatus } from "../types";

const styles: Record<string, string> = {
  matched: "bg-green-100 text-green-800",
  fuzzy_match: "bg-teal-100 text-teal-800",
  missing_from_settlement: "bg-red-100 text-red-800",
  unexpected_in_settlement: "bg-orange-100 text-orange-800",
  amount_mismatch: "bg-yellow-100 text-yellow-800",
  fee_mismatch: "bg-purple-100 text-purple-800",
  completed: "bg-green-100 text-green-800",
  running: "bg-blue-100 text-blue-800",
  pending: "bg-gray-100 text-gray-700",
  failed: "bg-red-100 text-red-800",
};

const labels: Record<string, string> = {
  matched: "Matched",
  fuzzy_match: "Fuzzy Match",
  missing_from_settlement: "Missing",
  unexpected_in_settlement: "Unexpected",
  amount_mismatch: "Amt Mismatch",
  fee_mismatch: "Fee Error",
  completed: "Completed",
  running: "Running",
  pending: "Pending",
  failed: "Failed",
};

interface Props {
  status: MatchStatus | string;
  small?: boolean;
}

export function StatusBadge({ status, small }: Props) {
  const cls = styles[status] ?? "bg-gray-100 text-gray-700";
  const label = labels[status] ?? status;
  return (
    <span
      className={`inline-flex items-center rounded-full font-medium ${small ? "px-2 py-0.5 text-xs" : "px-2.5 py-1 text-xs"} ${cls}`}
    >
      {label}
    </span>
  );
}
