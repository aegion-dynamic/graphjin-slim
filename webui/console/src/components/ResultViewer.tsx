interface Props {
  result: string
}

export function ResultViewer({ result }: Props) {
  return (
    <section className="bg-background flex min-h-0 flex-col">
      <div className="border-border bg-card flex items-center border-b px-3 py-2">
        <span className="text-muted-foreground text-xs tracking-wide uppercase">Result</span>
      </div>
      {result ? (
        <pre className="font-mono flex-1 overflow-auto p-3 text-sm whitespace-pre-wrap">
          {result}
        </pre>
      ) : (
        <p className="text-muted-foreground p-3 text-sm">Run a query to see results.</p>
      )}
    </section>
  )
}
