import { useState } from 'react';
import { RootType, skeletonFor } from '../schema';

interface Props {
  roots: RootType[];
  onPick: (query: string) => void;
}

// SchemaBrowser lists root query/mutation fields. Clicking a field inserts
// an editable skeleton into the editor.
export function SchemaBrowser({ roots, onPick }: Props) {
  const [active, setActive] = useState(roots[0]?.name ?? '');

  const current = roots.find((r) => r.name === active) ?? roots[0];

  return (
    <section className="pane schema">
      <div className="pane-head">
        {roots.map((r) => (
          <button
            key={r.name}
            className={r.name === current?.name ? 'tab active' : 'tab'}
            onClick={() => setActive(r.name)}
          >
            {r.name}
          </button>
        ))}
      </div>
      <ul>
        {current?.fields.map((f) => (
          <li key={f.name}>
            <button
              className="field"
              title={f.args.length ? f.args.map((a) => `$${a.name}: ${a.type}`).join(', ') : ''}
              onClick={() => onPick(skeletonFor(f))}
            >
              <span className="fname">{f.name}</span>
              <span className="ftype">{f.type}</span>
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}
