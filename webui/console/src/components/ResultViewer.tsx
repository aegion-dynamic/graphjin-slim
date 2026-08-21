interface Props {
  result: string;
}

export function ResultViewer({ result }: Props) {
  return (
    <section className="pane result">
      <div className="pane-head">
        <span>Result</span>
      </div>
      {result ? <pre>{result}</pre> : <p className="hint">Run a query to see results.</p>}
    </section>
  );
}
