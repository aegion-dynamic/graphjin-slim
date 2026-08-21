import { deleteQuery, fetchSavedQuery } from "../api"

export interface OpenedQuery {
  name: string
  query: string
  variables: string
}

interface Props {
  queries: { name: string; operation?: string }[]
  active: string
  onOpen: (q: OpenedQuery) => void
  onDelete: (name: string) => void
}

export function SavedQueries({ queries, active, onOpen, onDelete }: Props) {
  return (
    <aside className="rail">
      <div className="rail-head">
        <span>Saved Queries</span>
        <span className="rail-count">{queries.length}</span>
      </div>
      <ul className="rail-list">
        {queries.length === 0 && (
          <li className="rail-empty">
            Write a named query, then press Save.
            <br />
            It runs, persists, and its variables become defaults.
          </li>
        )}
        {queries.map((q) => (
          <li key={q.name}>
            <div
              className={q.name === active ? "rail-item rail-item-active" : "rail-item"}
              role="button"
              tabIndex={0}
              onClick={() => {
                void fetchSavedQuery(q.name)
                  .then((d) => onOpen({ name: q.name, ...d }))
                  .catch(alert)
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter") e.currentTarget.click()
              }}
            >
              <span className="rail-op">{opBadge(q.operation)}</span>
              <span className="rail-name">{q.name}</span>
              <span
                className="rail-delete"
                role="button"
                aria-label={`Delete ${q.name}`}
                onClick={(e) => {
                  e.stopPropagation()
                  if (confirm(`Delete saved query "${q.name}"?`)) {
                    void deleteQuery(q.name)
                      .then(() => onDelete(q.name))
                      .catch(alert)
                  }
                }}
              >
                ×
              </span>
            </div>
          </li>
        ))}
      </ul>
    </aside>
  )
}

function opBadge(op?: string): string {
  switch ((op ?? "").toLowerCase()) {
    case "mutation":
      return "M"
    case "subscription":
      return "S"
    default:
      return "Q"
  }
}
