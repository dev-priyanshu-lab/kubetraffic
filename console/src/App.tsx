import { Route, Routes } from "react-router-dom";
import { Layout } from "./components/Layout";
import { RoutesListPage } from "./pages/RoutesListPage";
import { RouteDetailPage } from "./pages/RouteDetailPage";
import { AuditLogPage } from "./pages/AuditLogPage";
import { PlaygroundPage } from "./pages/PlaygroundPage";

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<RoutesListPage />} />
        <Route path="/routes/:name" element={<RouteDetailPage />} />
        <Route path="/audit" element={<AuditLogPage />} />
        <Route path="/playground" element={<PlaygroundPage />} />
      </Route>
    </Routes>
  );
}
