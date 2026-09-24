import { useState } from "react";
import { api, ApiError } from "../lib/api";
import type { CanaryResponse, PolicyRule } from "../lib/types";
import { useApi } from "../lib/useApi";
import { ErrorView, LoadingView } from "./StateViews";

const STATUS_STYLES: Record<string, string> = {
  PROGRESSING: "bg-amber-950/40 text-amber-300 border-amber-800",
  PROMOTED: "bg-emerald-950/40 text-emerald-300 border-emerald-800",
  ROLLED_BACK: "bg-red-950/40 text-red-300 border-red-800",
};

export function CanaryPanel({
  name,
  namespace,
  rule,
}: {
  name: string;
  namespace: string;
  rule: PolicyRule;
}) {
  const path = rule.path;
  const versions = rule.versions?.map((v) => v.name) ?? [];

  const {
    data: canary,
    loading,
    error,
    reload,
  } = useApi<CanaryResponse | null>(
    () =>
      api.canary.get(name, namespace, path).catch((e) => {
        if (e instanceof ApiError && e.status === 404) return null;
        throw e;
      }),
    [name, namespace, path],
  );

  const [actionError, setActionError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function run(action: () => Promise<unknown>) {
    setBusy(true);
    setActionError(null);
    try {
      await action();
      reload();
    } catch (e) {
      setActionError(e instanceof ApiError ? e.message : "Action failed");
    } finally {
      setBusy(false);
    }
  }

  if (loading) return <LoadingView label="Loading canary state…" />;
  if (error) return <ErrorView message={error} onRetry={reload} />;

  return (
    <div className="rounded-lg border border-slate-800 p-4">
      <div className="flex items-center justify-between">
        <h3 className="font-medium">Canary — {path}</h3>
        {canary && (
          <span
            className={`rounded-full border px-2.5 py-0.5 text-xs font-medium ${STATUS_STYLES[canary.status]}`}
          >
            {canary.status}
          </span>
        )}
      </div>

      {actionError && (
        <div className="mt-3">
          <ErrorView message={actionError} />
        </div>
      )}

      {canary ? (
        <div className="mt-3 space-y-3 text-sm">
          <div className="grid grid-cols-2 gap-2 text-slate-300 sm:grid-cols-4">
            <Field label="Stable" value={canary.stableVersion} />
            <Field label="Canary" value={canary.canaryVersion} />
            <Field label="Weight" value={`${canary.currentCanaryWeight}%`} />
            <Field label="Step" value={`${canary.stepIndex + 1} / ${canary.steps.length}`} />
          </div>
          <div className="flex gap-1.5">
            {canary.steps.map((step, i) => (
              <span
                key={step}
                className={`h-1.5 flex-1 rounded-full ${
                  i <= canary.stepIndex ? "bg-sky-500" : "bg-slate-800"
                }`}
              />
            ))}
          </div>
          {canary.status === "PROGRESSING" && (
            <div className="flex gap-2 pt-1">
              <button
                disabled={busy}
                onClick={() => run(() => api.canary.promote(name, namespace, path))}
                className="rounded-md bg-sky-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-sky-500 disabled:opacity-50"
              >
                Advance / Promote
              </button>
              <button
                disabled={busy}
                onClick={() => run(() => api.canary.rollback(name, namespace, path))}
                className="rounded-md border border-red-800 px-3 py-1.5 text-xs font-medium text-red-300 hover:bg-red-950/40 disabled:opacity-50"
              >
                Rollback
              </button>
            </div>
          )}
        </div>
      ) : (
        <StartCanaryForm
          versions={versions}
          busy={busy}
          onStart={(stableVersion, canaryVersion) =>
            run(() => api.canary.start(name, { stableVersion, canaryVersion }, namespace, path))
          }
        />
      )}
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-xs text-slate-500">{label}</p>
      <p className="font-medium">{value}</p>
    </div>
  );
}

function StartCanaryForm({
  versions,
  busy,
  onStart,
}: {
  versions: string[];
  busy: boolean;
  onStart: (stable: string, canary: string) => void;
}) {
  const [stable, setStable] = useState(versions[0] ?? "");
  const [canaryVersion, setCanaryVersion] = useState(versions[1] ?? "");

  return (
    <div className="mt-3 space-y-3 text-sm">
      <p className="text-slate-400">No canary in progress for this path.</p>
      <div className="flex flex-wrap items-end gap-3">
        <VersionSelect label="Stable version" value={stable} onChange={setStable} versions={versions} />
        <VersionSelect
          label="Canary version"
          value={canaryVersion}
          onChange={setCanaryVersion}
          versions={versions}
        />
        <button
          disabled={busy || !stable || !canaryVersion || stable === canaryVersion}
          onClick={() => onStart(stable, canaryVersion)}
          className="rounded-md bg-sky-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-sky-500 disabled:opacity-50"
        >
          Start canary
        </button>
      </div>
    </div>
  );
}

function VersionSelect({
  label,
  value,
  onChange,
  versions,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  versions: string[];
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-xs text-slate-500">{label}</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="rounded-md border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm"
      >
        <option value="" disabled>
          Select…
        </option>
        {versions.map((v) => (
          <option key={v} value={v}>
            {v}
          </option>
        ))}
      </select>
    </label>
  );
}
