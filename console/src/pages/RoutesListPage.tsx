import { Link } from "react-router-dom";
import { api } from "../lib/api";
import { useApi } from "../lib/useApi";
import { EmptyView, ErrorView, LoadingView } from "../components/StateViews";

export function RoutesListPage() {
  const { data: policies, loading, error, reload } = useApi(() => api.policies.list(), []);

  return (
    <div>
      <h1 className="text-2xl font-semibold">Routes</h1>
      <p className="mt-1 text-sm text-slate-400">
        Traffic routes registered with the control plane, one per Kubernetes TrafficRoute.
      </p>

      <div className="mt-6">
        {loading && <LoadingView label="Loading routes…" />}
        {error && <ErrorView message={error} onRetry={reload} />}
        {!loading && !error && policies?.length === 0 && (
          <EmptyView
            title="No routes registered yet"
            hint="Apply a TrafficRoute custom resource to your cluster and it will appear here once the controller registers it."
          />
        )}
        {!loading && !error && policies && policies.length > 0 && (
          <div className="overflow-hidden rounded-lg border border-slate-800">
            <table className="w-full text-left text-sm">
              <thead className="bg-slate-900 text-slate-400">
                <tr>
                  <th className="px-4 py-2 font-medium">Name</th>
                  <th className="px-4 py-2 font-medium">Namespace</th>
                  <th className="px-4 py-2 font-medium">Host</th>
                  <th className="px-4 py-2 font-medium">Generation</th>
                  <th className="px-4 py-2 font-medium">Updated</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800">
                {policies.map((p) => (
                  <tr key={p.id} className="hover:bg-slate-900/60">
                    <td className="px-4 py-2.5">
                      <Link
                        to={`/routes/${encodeURIComponent(p.name)}?namespace=${encodeURIComponent(p.namespace)}`}
                        className="font-medium text-sky-400 hover:underline"
                      >
                        {p.name}
                      </Link>
                    </td>
                    <td className="px-4 py-2.5 text-slate-300">{p.namespace}</td>
                    <td className="px-4 py-2.5 text-slate-300">{p.spec.host ?? "—"}</td>
                    <td className="px-4 py-2.5 text-slate-300">{p.generation}</td>
                    <td className="px-4 py-2.5 text-slate-400">
                      {new Date(p.updatedAt).toLocaleString()}
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
