import { useEffect, useState } from "react"
import { endpointURL, runQuery } from "./api"
import { fetchRoots, RootType, skeletonFor } from "./schema"
import { SchemaBrowser } from "./components/SchemaBrowser"
import { ResultViewer } from "./components/ResultViewer"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"

export default function App() {
  const [roots, setRoots] = useState<RootType[]>([])
  const [error, setError] = useState("")
  const [query, setQuery] = useState("query Q {\n  \n}")
  const [result, setResult] = useState("")
  const [running, setRunning] = useState(false)

  useEffect(() => {
    fetchRoots()
      .then((r) => {
        setRoots(r)
        if (r.length && r[0].fields.length) {
          setQuery(skeletonFor(r[0].fields[0]))
        }
      })
      .catch((e: Error) => setError(e.message))
  }, [])

  async function execute() {
    setRunning(true)
    try {
      const res = await runQuery(query)
      setResult(JSON.stringify(res, null, 2))
      if (res.errors?.length) setError(res.errors[0].message)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setRunning(false)
    }
  }

  return (
    <div className="bg-background text-foreground flex h-screen flex-col">
      <header className="border-border bg-card flex items-center gap-3 border-b px-4 py-2.5">
        <h1 className="text-sm font-semibold">GraphJin Console</h1>
        <Badge variant="secondary" className="font-mono text-xs">
          {endpointURL()}
        </Badge>
        {error && (
          <span
            title={error}
            onClick={() => setError("")}
            className="text-destructive ml-auto max-w-1/2 cursor-pointer truncate text-xs"
          >
            ⚠ {error}
          </span>
        )}
      </header>
      {roots.length === 0 ? (
        <main className="text-muted-foreground grid flex-1 place-items-center">
          {error ? "" : <p>Introspecting schema…</p>}
        </main>
      ) : (
        <main className="grid min-h-0 flex-1 grid-cols-[260px_1fr_1fr] gap-px bg-border">
          <SchemaBrowser roots={roots} onPick={(q) => setQuery(q)} />
          <section className="bg-background flex min-h-0 flex-col">
            <div className="border-border bg-card flex items-center border-b px-3 py-2">
              <span className="text-muted-foreground text-xs tracking-wide uppercase">Query</span>
              <Button size="sm" className="ml-auto" disabled={running} onClick={execute}>
                {running ? "Running…" : "Run (Ctrl+↵)"}
              </Button>
            </div>
            <textarea
              spellCheck={false}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => {
                if ((e.ctrlKey || e.metaKey) && e.key === "Enter") execute()
              }}
              className="focus:ring-ring/50 font-mono flex-1 resize-none p-3 text-sm outline-none focus:ring-2"
            />
          </section>
          <ResultViewer result={result} />
        </main>
      )}
    </div>
  )
}
