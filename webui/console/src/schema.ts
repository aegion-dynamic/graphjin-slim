import { runQuery } from "./api"

// Minimal introspection model: only what the schema browser renders.

export interface Field {
  name: string
  type: string
  args: { name: string; type: string }[]
}

export interface RootType {
  name: string // "Queries" | "Mutations" | ...
  fields: Field[]
}

interface TypeRef {
  kind: string
  name: string | null
  ofType: TypeRef | null
}

interface IntrospectionResult {
  __schema: {
    queryType: { name: string }
    mutationType: { name: string } | null
    types: {
      name: string
      kind: string
      fields:
        | {
            name: string
            args: { name: string; type: TypeRef }[]
            type: TypeRef
          }[]
        | null
    }[]
  }
}

function typeRefName(t: TypeRef): string {
  if (t.kind === "NON_NULL") return typeRefName(t.ofType!) + "!"
  if (t.kind === "LIST") return `[${typeRefName(t.ofType!)}]`
  return t.name ?? "?"
}

// Introspection query. GraphJin supports standard introspection.
const INTROSPECTION = `query ConsoleIntrospection {
  __schema {
    queryType { name }
    mutationType { name }
    types {
      name
      kind
      fields {
        name
        type { kind name ofType { kind name ofType { kind name } } }
        args { name type { kind name ofType { kind name ofType { kind name } } } }
      }
    }
  }
}`

export async function fetchRoots(): Promise<RootType[]> {
  const res = await runQuery<IntrospectionResult>(INTROSPECTION)
  if (res.errors?.length || !res.data) {
    throw new Error(res.errors?.[0]?.message ?? "introspection failed")
  }
  const s = res.data.__schema
  const byName = new Map(s.types.map((t) => [t.name, t]))

  const build = (rootName: string | null): RootType | null => {
    if (!rootName) return null
    const t = byName.get(rootName)
    if (!t?.fields) return null
    return {
      name: rootName,
      fields: t.fields.map((f) => ({
        name: f.name,
        type: typeRefName(f.type),
        args: f.args.map((a) => ({ name: a.name, type: typeRefName(a.type) })),
      })),
    }
  }

  const roots = [build(s.queryType.name), build(s.mutationType?.name ?? null)]
  return roots.filter((r): r is RootType => r !== null)
}

// skeletonFor renders an editable starter query for a root field.
export function skeletonFor(field: Field): string {
  const args = field.args.map((a) => `$${a.name}: ${a.type.replace(/!$/, "")}`)
  const vars = field.args.filter((a) => a.type.endsWith("!")).map((a) => `$${a.name}: ${a.type}`)
  const varDecl = vars.length ? `(${vars.join(", ")})` : ""
  const argList = args.length ? `(${args.join(", ")})` : ""

  if (field.type.startsWith("[")) {
    return `query Q${varDecl} {\n  ${field.name}${argList} {\n    id\n  }\n}`
  }
  return `query Q${varDecl} {\n  ${field.name}${argList} {\n    id\n  }\n}`
}
