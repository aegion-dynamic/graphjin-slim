// Saved-query management API + GraphQL client helpers.

export function graphqlEndpoint(): string {
  const ep = new URLSearchParams(window.location.search).get("endpoint")
  return ep && ep.startsWith("/") ? ep : "/api/v1/graphql"
}

export interface GQLResponse<T> {
  data?: T
  errors?: { message: string }[]
}

export async function runQuery<T = unknown>(
  query: string,
  variables?: Record<string, unknown>,
): Promise<GQLResponse<T>> {
  const res = await fetch(graphqlEndpoint(), {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ query, variables: variables ?? {} }),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status} from ${graphqlEndpoint()}`)
  return res.json()
}

export function endpointURL(): string {
  return graphqlEndpoint()
}

export interface SavedQuerySummary {
  namespace?: string
  name: string
  operation?: string
}

const queriesAPI = "/api/v1/queries"

export async function listSavedQueries(): Promise<SavedQuerySummary[]> {
  const res = await fetch(queriesAPI)
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  const body = await res.json()
  return body.queries ?? []
}

export interface SaveQueryInput {
  name: string
  query: string
  operation?: string
  variables?: Record<string, unknown>
}

export async function saveQuery(input: SaveQueryInput): Promise<void> {
  const res = await fetch(queriesAPI, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `HTTP ${res.status}`)
  }
}

export async function deleteQuery(name: string): Promise<void> {
  const res = await fetch(`${queriesAPI}?name=${encodeURIComponent(name)}`, {
    method: "DELETE",
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `HTTP ${res.status}`)
  }
}

/** Fetch one saved query's source and default variables. */
export async function fetchSavedQuery(name: string): Promise<{ query: string; variables: string }> {
  const res = await fetch(`${queriesAPI}?name=${encodeURIComponent(name)}`)
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  const d = await res.json()
  return {
    query: d.query ?? "",
    variables: d.variables ? JSON.stringify(d.variables, null, 2) : "",
  }
}
