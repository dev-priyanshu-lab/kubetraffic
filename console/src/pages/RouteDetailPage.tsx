import { Link, useParams, useSearchParams } from "react-router-dom";
import { api } from "../lib/api";
import { useApi } from "../lib/useApi";
import { EmptyView, ErrorView, LoadingView } from "../components/StateViews";
import { CanaryPanel } from "../components/CanaryPanel";

export function RouteDetailPage() {
  const { name = "" } = useParams();
  const [searchParams] = useSearchParams();
  const namespace = searchParams.get("namespace") ?? "default";

  const { data: policy, loading: policyLoading, error: policyError, reload: reloadPolicy } = useApi(
    () => api.policies.get(name, namespace),
    [name, namespace],
  );
  const { data: routeConfig, loading: rcLoading, error: rcError, reload: reloadRc } = useApi(
    () => api.policies.routeConfig(name, namespace),
    [name, namespace],
  );
  const { data: auditRecords } = useApi(
    () => api.audit.query(`policy/${namespace}/${name}`, 20),
    [name, namespace],
  );

  const loading = policyLoading || rcLoading;
  const error = policyError ?? rcError;

  return (
    <div>
      <Link to="/" className="text-sm text-slate-400 hover:text-slate-200">
        ← Routes
      </Link>
      <h1 className="mt-2 text-2xl font-semibold">{name}</h1>
      <p className="mt-1 text-sm text-slate-400">
        {namespace} {policy?.spec.host && `· ${policy.spec.host}`}
      </p>

      {loading && <LoadingView label="Loading route…" />}
      {error && (
        <div className="mt-4">
          <ErrorView
            message={error}
            onRetry={() => {
              reloadPolicy();
              reloadRc();
            }}
          />
        </div>
      )}

      {!loading && !error && policy && (
        <div className="mt-6 space-y-6">
          {(policy.spec.rules ?? []).length === 0 && (
            <EmptyView title="This route has no rules defined" />
          )}

          {(policy.spec.rules ?? []).map((rule) => {
            const effective = routeConfig?.rules.find((r) => r.path === rule.path);
            return (
              <section key={rule.path} className="space-y-4">
                <div className="rounded-lg border border-slate-800 p-4">
                  <div className="flex items-center justify-between">
                    <h2 className="font-medium">{rule.path}</h2>
                    <span className="text-xs text-slate-500">
                      {rule.backendService}:{rule.backendPort}
                    </span>
                  </div>
                  <div className="mt-3 space-y-2">
                    {(
                      effective?.weights ??
                      (rule.versions ?? []).map((v) => ({ version: v.name, weight: v.weight }))
                    ).map((v) => (
                      <div key={v.version} className="flex items-center gap-3">
                        <span className="w-16 text-sm text-slate-300">{v.version}</span>
                        <div className="h-2 flex-1 overflow-hidden rounded-full bg-slate-800">
                          <div
                            className="h-full rounded-full bg-sky-500"
                            style={{ width: `${v.weight}%` }}
                          />
                        </div>
                        <span className="w-10 text-right text-sm text-slate-400">{v.weight}%</span>
                      </div>
                    ))}
                  </div>
                </div>

                {(rule.versions?.length ?? 0) >= 2 && (
                  <CanaryPanel name={name} namespace={namespace} rule={rule} />
                )}
              </section>
            );
          })}

          <section>
            <h2 className="font-medium">Recent activity</h2>
            <div className="mt-3">
              {!auditRecords || auditRecords.length === 0 ? (
                <EmptyView title="No audit entries yet for this route" />
              ) : (
                <ul className="divide-y divide-slate-800 rounded-lg border border-slate-800">
                  {auditRecords.map((record) => (
                    <li key={record.id} className="px-4 py-2.5 text-sm">
                      <div className="flex items-center justify-between">
                        <span className="font-medium text-slate-200">{record.action}</span>
                        <span className="text-xs text-slate-500">
                          {new Date(record.createdAt).toLocaleString()}
                        </span>
                      </div>
                      <p className="text-xs text-slate-500">by {record.actor}</p>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </section>
        </div>
      )}
    </div>
  );
}
