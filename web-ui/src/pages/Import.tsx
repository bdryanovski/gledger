import { useState, useRef } from "react";

interface ImportResult {
  imported: number;
  skipped: number;
  errors: string[];
  total: number;
  dry_run: boolean;
}

export default function Import() {
  const [mapFile, setMapFile] = useState<File | null>(null);
  const [csvFile, setCsvFile] = useState<File | null>(null);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const mapRef = useRef<HTMLInputElement>(null);
  const csvRef = useRef<HTMLInputElement>(null);

  const doImport = async (dryRun: boolean) => {
    if (!mapFile || !csvFile) return;
    setLoading(true);
    setError(null);
    setResult(null);
    const form = new FormData();
    form.append("map", mapFile);
    form.append("csv", csvFile);
    form.append("dry_run", String(dryRun));
    try {
      const res = await fetch("/api/import", { method: "POST", body: form });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || res.statusText);
      setResult(data as ImportResult);
    } catch (e) {
      setError(String(e instanceof Error ? e.message : e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="p-6 space-y-6 max-w-2xl">
      <h1 className="text-2xl font-bold text-gray-900">Import CSV</h1>

      {/* Step 1 */}
      <Step number={1} title="Select Import Map (.importmap.json)">
        <FileInput
          ref={mapRef}
          accept=".json"
          label="Drop importmap.json here or click to browse"
          file={mapFile}
          onFile={setMapFile}
        />
      </Step>

      {/* Step 2 */}
      <Step number={2} title="Select CSV File">
        <FileInput
          ref={csvRef}
          accept=".csv,.txt"
          label="Drop CSV file here or click to browse"
          file={csvFile}
          onFile={setCsvFile}
        />
      </Step>

      {/* Actions */}
      <div className="flex gap-3">
        <button
          onClick={() => doImport(true)}
          disabled={!mapFile || !csvFile || loading}
          className="px-5 py-2.5 border border-purple-300 text-purple-700 rounded-lg text-sm hover:bg-purple-50 disabled:opacity-40"
        >
          Dry Run (preview)
        </button>
        <button
          onClick={() => doImport(false)}
          disabled={!mapFile || !csvFile || loading}
          className="px-5 py-2.5 bg-purple-600 text-white rounded-lg text-sm hover:bg-purple-700 disabled:opacity-40"
        >
          {loading ? "Importing…" : "Import"}
        </button>
      </div>

      {/* Error */}
      {error && (
        <div className="bg-red-50 border border-red-200 rounded-xl p-4 text-red-700 text-sm">
          {error}
        </div>
      )}

      {/* Result */}
      {result && (
        <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-5 space-y-3">
          <h2 className="font-semibold text-gray-800">
            {result.dry_run ? "Dry Run Results" : "Import Complete"}
          </h2>
          <div className="grid grid-cols-2 gap-3 text-sm">
            <Stat label="Rows processed" value={result.total} />
            <Stat
              label={result.dry_run ? "Would import" : "Imported"}
              value={result.imported}
              color="text-green-700"
            />
            <Stat label="Skipped (duplicates)" value={result.skipped} color="text-gray-500" />
            <Stat label="Errors" value={result.errors.length} color={result.errors.length ? "text-red-600" : undefined} />
          </div>
          {result.errors.length > 0 && (
            <div className="mt-3">
              <p className="text-xs font-semibold text-gray-500 uppercase mb-2">Warnings</p>
              <ul className="space-y-1">
                {result.errors.map((e, i) => (
                  <li key={i} className="text-xs text-red-600 font-mono bg-red-50 rounded px-2 py-1">
                    {e}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function Step({ number, title, children }: { number: number; title: string; children: React.ReactNode }) {
  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <span className="w-6 h-6 rounded-full bg-purple-600 text-white text-xs flex items-center justify-center font-bold">
          {number}
        </span>
        <h2 className="font-semibold text-gray-800 text-sm">{title}</h2>
      </div>
      {children}
    </div>
  );
}

const FileInput = ({
  ref,
  accept,
  label,
  file,
  onFile,
}: {
  ref: React.RefObject<HTMLInputElement | null>;
  accept: string;
  label: string;
  file: File | null;
  onFile: (f: File) => void;
}) => (
  <div
    className={`border-2 border-dashed rounded-xl p-6 text-center cursor-pointer transition-colors ${
      file ? "border-purple-400 bg-purple-50" : "border-gray-200 hover:border-purple-300 hover:bg-gray-50"
    }`}
    onClick={() => ref.current?.click()}
    onDragOver={(e) => e.preventDefault()}
    onDrop={(e) => {
      e.preventDefault();
      const f = e.dataTransfer.files[0];
      if (f) onFile(f);
    }}
  >
    <input
      ref={ref}
      type="file"
      accept={accept}
      className="hidden"
      onChange={(e) => { if (e.target.files?.[0]) onFile(e.target.files[0]); }}
    />
    {file ? (
      <p className="text-sm text-purple-700 font-medium">✓ {file.name}</p>
    ) : (
      <p className="text-sm text-gray-400">{label}</p>
    )}
  </div>
);

function Stat({ label, value, color = "text-gray-900" }: { label: string; value: number; color?: string }) {
  return (
    <div className="bg-gray-50 rounded-lg p-3">
      <p className="text-xs text-gray-500">{label}</p>
      <p className={`text-xl font-bold mt-1 ${color}`}>{value}</p>
    </div>
  );
}
