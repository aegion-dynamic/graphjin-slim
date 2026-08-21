import { GraphiQL } from "graphiql"
import { createGraphiQLFetcher } from "@graphiql/toolkit"
import "graphiql/style.css"

interface Props {
  /** Initial contents; applied when instanceKey changes (remount). */
  query: string
  variables: string
  instanceKey: number
  onQueryChange: (q: string) => void
  onVariablesChange: (v: string) => void
}

// QueryEditor wraps GraphiQL: schema-aware autocomplete, inline validation,
// a variables editor, and docs exploration against the live endpoint.
export function QueryEditor({
  query,
  variables,
  instanceKey,
  onQueryChange,
  onVariablesChange,
}: Props) {
  const fetcher = createGraphiQLFetcher({
    url: `${window.location.origin}${endpointPath()}`,
  })

  return (
    <div className="graphiql-shell">
      <GraphiQL
        key={instanceKey}
        fetcher={fetcher}
        initialQuery={query}
        initialVariables={variables}
        onEditQuery={onQueryChange}
        onEditVariables={onVariablesChange}
        defaultEditorToolsVisibility="variables"
        isHeadersEditorEnabled={false}
      />
    </div>
  )
}

function endpointPath(): string {
  const ep = new URLSearchParams(window.location.search).get("endpoint")
  return ep && ep.startsWith("/") && !ep.startsWith("//") ? ep : "/api/v1/graphql"
}
