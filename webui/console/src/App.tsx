import { useCallback, useEffect, useState } from "react"
import { QueryEditor } from "./components/QueryEditor"
import { SavedQueries, OpenedQuery } from "./components/SavedQueries"
import { endpointURL, listSavedQueries } from "./api"

const DEFAULT_QUERY = `# GraphJin — write a named query and hit Save.
# Autocomplete: Ctrl+Space. Validation happens as you type.

query getUsers {
  users(limit: 10, order_by: { id: desc }) {
    id
    full_name
    email
    products {
      name
      price
    }
  }
}
`

function extractOpName(query: string): string {
  const m = query.match(/\b(query|mutation|subscription)\s+([A-Za-z_][A-Za-z0-9_]*)/)
  return m ? m[2] : ""
}

// GraphiQL persists editor tabs in localStorage; clear it so remounts always
// start from the content we pass in.
function resetGraphiQLStorage() {
  for (const key of Object.keys(localStorage)) {
    if (key.startsWith("graphiql")) localStorage.removeItem(key)
  }
}

export default function App() {
  const [queries, setQueries] = useState<{ name: string; operation?: string }[]>([])
  const [active, setActive] = useState("")
  const [query, setQuery] = useState(DEFAULT_QUERY)
  const [variables, setVariables] = useState("{}")
  const [instanceKey, setInstanceKey] = useState(0)
  const [name, setName] = useState("getUsers")
  const [status, setStatus] = useState("")

  const refresh = useCallback(() => {
    listSavedQueries()
      .then(setQueries)
      .catch(() => {})
  }, [])

  useEffect(refresh, [refresh])

  useEffect(() => {
    const n = extractOpName(query)
    if (n) setName(n)
  }, [query])

  function save() {
    if (!name.trim()) return void setStatus("Query needs a name")
    let vars: Record<string, unknown> = {}
    try {
      vars = JSON.parse(variables || "{}")
    } catch {
      return void setStatus("Variables are not valid JSON")
    }
    fetch("/api/v1/queries", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: name.trim(), query, variables: vars }),
    })
      .then(async (r) => {
        if (!r.ok) throw new Error((await r.json().catch(() => null))?.error ?? `HTTP ${r.status}`)
        setActive(name.trim())
        setStatus(`Saved "${name.trim()}"`)
        refresh()
      })
      .catch((e: Error) => setStatus(e.message))
  }

  function onOpen(q: OpenedQuery) {
    resetGraphiQLStorage()
    setActive(q.name)
    setQuery(q.query || DEFAULT_QUERY)
    setVariables(q.variables || "{}")
    setInstanceKey((k) => k + 1)
    setStatus(`Loaded "${q.name}"`)
  }

  function onDelete(deleted: string) {
    refresh()
    if (active === deleted) setActive("")
    setStatus(`Deleted "${deleted}"`)
  }

  return (
    <div className="app">
      <header className="topbar">
        <div className="brand">
          <span className="brand-dot" />
          <span className="brand-name">GraphJin</span>
          <span className="brand-env">console</span>
        </div>
        <span className="endpoint">{endpointURL()}</span>
        {status && <span className="status">{status}</span>}
        <div className="actions">
          <input
            className="name-input"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="query-name"
            spellCheck={false}
          />
          <button className="save-btn" onClick={save}>
            Save
          </button>
        </div>
      </header>
      <div className="body">
        <SavedQueries queries={queries} active={active} onOpen={onOpen} onDelete={onDelete} />
        <main className="editor">
          <QueryEditor
            query={query}
            variables={variables}
            instanceKey={instanceKey}
            onQueryChange={setQuery}
            onVariablesChange={setVariables}
          />
        </main>
      </div>
    </div>
  )
}
