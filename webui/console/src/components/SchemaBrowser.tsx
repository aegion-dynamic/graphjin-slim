import { useState } from "react"
import { RootType, skeletonFor } from "../schema"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Badge } from "@/components/ui/badge"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"

interface Props {
  roots: RootType[]
  onPick: (query: string) => void
}

// SchemaBrowser lists root query/mutation fields in tabs. Clicking a field
// inserts an editable skeleton into the editor.
export function SchemaBrowser({ roots, onPick }: Props) {
  const [active, setActive] = useState(roots[0]?.name ?? "")
  const current = roots.find((r) => r.name === active) ?? roots[0]

  return (
    <section className="bg-background flex min-h-0 flex-col">
      <Tabs value={active} onValueChange={(v) => setActive(v)}>
        <TabsList className="w-full px-1 pt-1">
          {roots.map((r) => (
            <TabsTrigger key={r.name} value={r.name}>
              {r.name}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>
      <ul className="flex-1 overflow-y-auto p-1.5">
        {current?.fields.map((f) => (
          <li key={f.name}>
            <TooltipProvider delay={200}>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <button
                      className="hover:bg-accent flex w-full items-center justify-between gap-2 rounded-md px-2 py-1.5 text-left"
                      onClick={() => onPick(skeletonFor(f))}
                    >
                      <span className="font-mono text-sm">{f.name}</span>
                      <Badge
                        variant="outline"
                        className="text-muted-foreground shrink-0 font-mono text-[11px]"
                      >
                        {f.type}
                      </Badge>
                    </button>
                  }
                />
                {f.args.length > 0 && (
                  <TooltipContent side="right" align="start">
                    <code className="text-xs">
                      ({f.args.map((a) => `$${a.name}: ${a.type}`).join(", ")})
                    </code>
                  </TooltipContent>
                )}
              </Tooltip>
            </TooltipProvider>
          </li>
        ))}
      </ul>
    </section>
  )
}
