import { useState } from "react";
import { api, ApiError } from "../lib/api";
import type { CircuitBreakerRecord, RateLimitResult } from "../lib/types";
import { ErrorView } from "../components/StateViews";

export function PlaygroundPage() {
  return (
    <div>
      <h1 className="text-2xl font-semibold">Playground</h1>
      <p className="mt-1 text-sm text-slate-400">
        Exercise the shared rate-limiter and circuit-breaker primitives directly — useful for
        understanding their behavior before wiring real traffic through them.
      </p>

      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        <RateLimitCard />
        <CircuitBreakerCard />
      </div>
    </div>
  );
}

function RateLimitCard() {
  const [key, setKey] = useState("demo:playground");
  const [limit, setLimit] = useState(10);
  const [windowSeconds, setWindowSeconds] = useState(60);
  const [result, setResult] = useState<RateLimitResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function check() {
    setBusy(true);
    setError(null);
    try {
      setResult(await api.rateLimit.check(key, limit, windowSeconds));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Request failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="rounded-lg border border-slate-800 p-4">
      <h2 className="font-medium">Rate limiter</h2>
      <div className="mt-3 space-y-3 text-sm">
        <TextField label="Key" value={key} onChange={setKey} />
        <div className="flex gap-3">
          <NumberField label="Limit" value={limit} onChange={setLimit} />
          <NumberField label="Window (s)" value={windowSeconds} onChange={setWindowSeconds} />
        </div>
        <button
          disabled={busy}
          onClick={check}
          className="rounded-md bg-sky-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-sky-500 disabled:opacity-50"
        >
          Check
        </button>
        {error && <ErrorView message={error} />}
        {result && (
          <div
            className={`rounded-md border px-3 py-2 text-sm ${
              result.allowed
                ? "border-emerald-800 bg-emerald-950/30 text-emerald-300"
                : "border-red-800 bg-red-950/30 text-red-300"
            }`}
          >
            {result.allowed ? "Allowed" : "Rejected (429)"} — {result.count}/{result.limit} used,{" "}
            {result.remaining} remaining
          </div>
        )}
      </div>
    </div>
  );
}

function CircuitBreakerCard() {
  const [key, setKey] = useState("demo:playground:v2");
  const [threshold, setThreshold] = useState(5);
  const [record, setRecord] = useState<CircuitBreakerRecord | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function run(action: () => Promise<CircuitBreakerRecord>) {
    setBusy(true);
    setError(null);
    try {
      setRecord(await action());
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Request failed");
    } finally {
      setBusy(false);
    }
  }

  const stateStyles: Record<string, string> = {
    CLOSED: "border-emerald-800 bg-emerald-950/30 text-emerald-300",
    OPEN: "border-red-800 bg-red-950/30 text-red-300",
    HALF_OPEN: "border-amber-800 bg-amber-950/30 text-amber-300",
  };

  return (
    <div className="rounded-lg border border-slate-800 p-4">
      <h2 className="font-medium">Circuit breaker</h2>
      <div className="mt-3 space-y-3 text-sm">
        <TextField label="Key" value={key} onChange={setKey} />
        <NumberField label="Failure threshold" value={threshold} onChange={setThreshold} />
        <div className="flex gap-2">
          <button
            disabled={busy}
            onClick={() => run(() => api.circuitBreaker.get(key))}
            className="rounded-md border border-slate-700 px-3 py-1.5 text-xs font-medium hover:bg-slate-900 disabled:opacity-50"
          >
            Refresh
          </button>
          <button
            disabled={busy}
            onClick={() => run(() => api.circuitBreaker.recordFailure(key, threshold))}
            className="rounded-md border border-red-800 px-3 py-1.5 text-xs font-medium text-red-300 hover:bg-red-950/40 disabled:opacity-50"
          >
            Record failure
          </button>
          <button
            disabled={busy}
            onClick={() => run(() => api.circuitBreaker.recordSuccess(key))}
            className="rounded-md border border-emerald-800 px-3 py-1.5 text-xs font-medium text-emerald-300 hover:bg-emerald-950/40 disabled:opacity-50"
          >
            Record success
          </button>
        </div>
        {error && <ErrorView message={error} />}
        {record && (
          <div className={`rounded-md border px-3 py-2 text-sm ${stateStyles[record.state]}`}>
            {record.state} — {record.consecutiveFailures} consecutive failure(s)
            {record.openedAt && <> · opened {new Date(record.openedAt).toLocaleTimeString()}</>}
          </div>
        )}
      </div>
    </div>
  );
}

function TextField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-xs text-slate-500">{label}</span>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="rounded-md border border-slate-700 bg-slate-900 px-2 py-1.5"
      />
    </label>
  );
}

function NumberField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: number;
  onChange: (v: number) => void;
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-xs text-slate-500">{label}</span>
      <input
        type="number"
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="w-28 rounded-md border border-slate-700 bg-slate-900 px-2 py-1.5"
      />
    </label>
  );
}
