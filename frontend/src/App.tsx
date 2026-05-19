import { Routes, Route, Navigate } from "react-router-dom";
import { Layout } from "./components/Layout";
import { DashboardPage } from "./pages/DashboardPage";
import { TransactionsPage } from "./pages/TransactionsPage";
import { SettlementsPage } from "./pages/SettlementsPage";
import { ReconciliationPage } from "./pages/ReconciliationPage";
import { RunDetailPage } from "./pages/RunDetailPage";

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<Navigate to="/dashboard" replace />} />
        <Route path="dashboard" element={<DashboardPage />} />
        <Route path="transactions" element={<TransactionsPage />} />
        <Route path="settlements" element={<SettlementsPage />} />
        <Route path="reconciliation" element={<ReconciliationPage />} />
        <Route path="reconciliation/:id" element={<RunDetailPage />} />
      </Route>
    </Routes>
  );
}
