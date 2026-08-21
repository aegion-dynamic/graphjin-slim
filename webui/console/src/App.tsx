import { useCallback, useEffect, useState } from "react"
import { GraphiQL } from "graphiql"
import { ToolbarButton, ToolbarMenu } from "@graphiql/react"
import { createGraphiQLFetcher } from "@graphiql/toolkit"
import "graphiql/graphiql.css"

const DEFAULT_QUERY = `query getUsers {
  users(limit: 10, orderBy: { id: desc }) {
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

// Content for tabs opened via the "+" button inside GraphiQL.
const WELCOME_QUERY = `# GraphJin Slim — write GraphQL, get SQL.
#
#     query getUsers {
#       users(limit: 10, orderBy: { id: desc }) {
#         id
#         full_name
#       }
#     }
#
# Run Query:     Ctrl-Enter (or the play button)
# Auto Complete: Ctrl-Space (or just start typing)
#
# Name an operation and press Save to persist it with its variables.
`

function endpointPath(): string {
  const ep = new URLSearchParams(window.location.search).get("endpoint")
  return ep && ep.startsWith("/") && !ep.startsWith("//") ? ep : "/api/v1/graphql"
}

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

interface SavedQuerySummary {
  name: string
  operation?: string
}

export default function App() {
  const [queries, setQueries] = useState<SavedQuerySummary[]>([])
  const [active, setActive] = useState("")
  const [query, setQuery] = useState(DEFAULT_QUERY)
  const [variables, setVariables] = useState("{}")
  const [instanceKey, setInstanceKey] = useState(0)
  const [status, setStatus] = useState("")

  const fetcher = createGraphiQLFetcher({
    url: `${window.location.origin}${endpointPath()}`,
  })

  const refresh = useCallback(() => {
    listSaved()
      .then(setQueries)
      .catch(() => {})
  }, [])

  useEffect(refresh, [refresh])

  useEffect(() => {
    const t = setTimeout(() => setStatus(""), 2500)
    return () => clearTimeout(t)
  }, [status])

  function save() {
    const name = extractOpName(query)
    if (!name) return void setStatus("Name the operation to save it")
    let vars: Record<string, unknown> = {}
    try {
      vars = JSON.parse(variables || "{}")
    } catch {
      return void setStatus("Variables are not valid JSON")
    }
    fetch("/api/v1/queries", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, query, variables: vars }),
    })
      .then(async (r) => {
        if (!r.ok) throw new Error((await r.json().catch(() => null))?.error ?? `HTTP ${r.status}`)
        setActive(name)
        setStatus(`Saved "${name}"`)
        refresh()
      })
      .catch((e: Error) => setStatus(e.message))
  }

  function open(name: string) {
    fetch(`/api/v1/queries?name=${encodeURIComponent(name)}`)
      .then((r) => r.json())
      .then((d) => {
        resetGraphiQLStorage()
        setActive(name)
        setQuery(d.query || DEFAULT_QUERY)
        setVariables(d.variables ? JSON.stringify(d.variables, null, 2) : "{}")
        setInstanceKey((k) => k + 1)
        setStatus(`Loaded "${name}"`)
      })
      .catch(() => setStatus(`Failed to load "${name}"`))
  }

  function remove(name: string) {
    if (!confirm(`Delete saved query "${name}"?`)) return
    fetch(`/api/v1/queries?name=${encodeURIComponent(name)}`, {
      method: "DELETE",
    })
      .then(() => {
        refresh()
        if (active === name) setActive("")
        setStatus(`Deleted "${name}"`)
      })
      .catch(() => setStatus(`Failed to delete "${name}"`))
  }

  return (
    <div className="app">
      <GraphiQL
        key={instanceKey}
        fetcher={fetcher}
        query={query}
        variables={variables}
        onEditQuery={setQuery}
        onEditVariables={setVariables}
        isHeadersEditorEnabled={false}
        defaultQuery={WELCOME_QUERY}
      >
        <GraphiQL.Logo>{null}</GraphiQL.Logo>
        <GraphiQL.Toolbar>
          <ToolbarMenu
            label="Saved Queries"
            button={
              <span className="gj-menu-btn" title="Saved Queries">
                Saved…
              </span>
            }
          >
            {queries.length === 0 && (
              <ToolbarMenu.Item disabled>No saved queries yet</ToolbarMenu.Item>
            )}
            {queries.map((q) => (
              <ToolbarMenu.Item key={q.name} onSelect={() => open(q.name)} title={q.operation}>
                {badge(q.operation)} {q.name}
              </ToolbarMenu.Item>
            ))}
            {active && queries.length > 0 && (
              <ToolbarMenu.Item onSelect={() => remove(active)}>
                Delete "{active}"…
              </ToolbarMenu.Item>
            )}
          </ToolbarMenu>
          <ToolbarButton onClick={save} label="Save named query with its variables">
            Save
          </ToolbarButton>
        </GraphiQL.Toolbar>
      </GraphiQL>
    </div>
  )
}

function badge(op?: string): string {
  switch ((op ?? "").toLowerCase()) {
    case "mutation":
      return "[M]"
    case "subscription":
      return "[S]"
    default:
      return "[Q]"
  }
}

async function listSaved(): Promise<SavedQuerySummary[]> {
  const res = await fetch("/api/v1/queries")
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  const body = await res.json()
  return body.queries ?? []
}
