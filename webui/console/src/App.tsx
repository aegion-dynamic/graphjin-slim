import { useEffect, useState } from 'react';
import { endpointURL, runQuery } from './api';
import { fetchRoots, RootType, skeletonFor } from './schema';
import { SchemaBrowser } from './components/SchemaBrowser';
import { ResultViewer } from './components/ResultViewer';

export default function App() {
  const [roots, setRoots] = useState<RootType[]>([]);
  const [error, setError] = useState('');
  const [query, setQuery] = useState(
    'query Q {\n  \n}',
  );
  const [result, setResult] = useState('');
  const [running, setRunning] = useState(false);

  useEffect(() => {
    fetchRoots()
      .then((r) => {
        setRoots(r);
        // Seed the editor with the first root field as a starting point.
        if (r.length && r[0].fields.length && query.trim() === 'query Q {\n  \n}') {
          setQuery(skeletonFor(r[0].fields[0]));
        }
      })
      .catch((e: Error) => setError(e.message));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function execute() {
    setRunning(true);
    try {
      const res = await runQuery(query);
      setResult(JSON.stringify(res, null, 2));
      if (res.errors?.length) setError(res.errors[0].message);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setRunning(false);
    }
  }

  return (
    <div className="app">
      <header>
        <h1>GraphJin Console</h1>
        <span className="endpoint">{endpointURL()}</span>
        {error && (
          <span className="error" title={error} onClick={() => setError('')}>
            ⚠ {error}
          </span>
        )}
      </header>
      <main className={roots.length ? '' : 'loading'}>
        {!roots.length && !error && <p>Introspecting schema…</p>}
        {roots.length > 0 && (
          <>
            <SchemaBrowser
              roots={roots}
              onPick={(q) => setQuery(q)}
            />
            <section className="pane editor">
              <div className="pane-head">
                <span>Query</span>
                <button onClick={execute} disabled={running}>
                  {running ? 'Running…' : 'Run (Ctrl+↵)'}
                </button>
              </div>
              <textarea
                spellCheck={false}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={(e) => {
                  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') execute();
                }}
              />
            </section>
            <ResultViewer result={result} />
          </>
        )}
      </main>
    </div>
  );
}
