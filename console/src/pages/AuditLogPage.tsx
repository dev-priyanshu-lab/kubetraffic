import { useState } from "react";
import { api } from "../lib/api";
import { useApi } from "../lib/useApi";
import { EmptyView, ErrorView, LoadingView } from "../components/StateViews";

export function AuditLogPage() {
  const [target, setTarget] = useState("");
  const [submittedTarget, setSubmittedTarget] = useState<string | undefined>(undefined);

  const { data: records, loading, error, reload } = useApi(
    () => api.audit.query(submittedTarget, 200),
    [submittedTarget],
  );

  return (
    <div>
      <h1 className="text-2xl font-semibold">Audit Log</h1>
      <p className="mt-1 text-sm text-slate-400">
        Every mutation the control plane recorded — policy changes, canary transitions, deletes.
      </p>

      <form
        className="mt-4 flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          setSubmittedTarget(target.trim() || undefined);
        }}
      >
        <input
          value={target}
          onChange={(e) => setTarget(e.target.value)}
          placeholder="Filter by target, e.g. policy/default/payment-route"
          className="w-96 rounded-md border border-slate-700 bg-slate-900 px-3 py-1.5 text-sm placeholder:text-slate-600"
        />
        <button
          type="submit"
          className="rounded-md border border-slate-700 px-3 py-1.5 text-sm hover:bg-slate-900"
        >
          Filter
        </button>
      </form>

      <div className="mt-6">
        {loading && <LoadingView label="Loading audit entries…" />}
        {error && <ErrorView message={error} onRetry={reload} />}
        {!loading && !error && records?.length === 0 && (
          <EmptyView title="No audit entries match" hint="Try clearing the filter." />
        )}
        {!loading && !error && records && records.length > 0 && (
          <div className="overflow-hidden rounded-lg border border-slate-800">
            <table className="w-full text-left text-sm">
              <thead className="bg-slate-900 text-slate-400">
                <tr>
                  <th className="px-4 py-2 font-medium">Time</th>
                  <th className="px-4 py-2 font-medium">Actor</th>
                  <th className="px-4 py-2 font-medium">Action</th>
                  <th className="px-4 py-2 font-medium">Target</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800">
                {records.map((r) => (
                  <tr key={r.id} className="hover:bg-slate-900/60">
                    <td className="px-4 py-2.5 text-slate-400">{new Date(r.createdAt).toLocaleString()}</td>
                    <td className="px-4 py-2.5">{r.actor}</td>
                    <td className="px-4 py-2.5 font-medium text-slate-200">{r.action}</td>
                    <td className="px-4 py-2.5 text-slate-400">{r.target}</td>
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
