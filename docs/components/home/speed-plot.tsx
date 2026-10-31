// Mean of 3 runs from docs/content/docs/benchmarks.mdx (Apple M1 Pro, Go 1.25).
const rows = [
  { name: 'Nested query', shape: 'post(id) → author', grx: 9.49, gophers: 18.56, graphqlGo: 120.2 },
  { name: 'Feed timeline', shape: 'feed → posts → author', grx: 52.5, gophers: 102.0, graphqlGo: 236.0 },
  { name: 'Persisted compound', shape: 'fragments, aliases, variables', grx: 66.8, gophers: 101.3, graphqlGo: 275.8 },
];

const MAX = 300;
const pct = (us: number) => `${(us / MAX) * 100}%`;

export function SpeedPlot() {
  return (
    <div className="rounded-lg border border-fd-border bg-fd-card p-5 sm:p-8">
      <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-xs text-fd-muted-foreground">
        <span className="inline-flex items-center gap-2">
          <span className="size-3 rounded-full bg-grx-500 shadow-[0_0_0_4px_rgba(99,102,241,0.18)]" /> grx
        </span>
        <span className="inline-flex items-center gap-2">
          <span className="size-3 rounded-full border-2 border-grx-400 bg-fd-card" /> graph-gophers
        </span>
        <span className="inline-flex items-center gap-2">
          <span className="size-3 rounded-full bg-fd-muted-foreground/45" /> graphql-go
        </span>
        <span className="ms-auto font-mono">µs per op · lower is better</span>
      </div>

      <div className="mt-8 space-y-9">
        {rows.map((r) => (
          <div key={r.name} className="grid gap-3 md:grid-cols-[11rem_1fr_6rem] md:items-center md:gap-6">
            <div>
              <p className="font-medium">{r.name}</p>
              <p className="font-mono text-[11px] text-fd-muted-foreground">{r.shape}</p>
            </div>
            <div className="relative h-8">
              <div className="absolute inset-x-0 top-1/2 h-px -translate-y-1/2 bg-fd-border" />
              <div
                className="absolute top-1/2 h-[3px] -translate-y-1/2 rounded-full bg-gradient-to-r from-grx-500 to-grx-500/10"
                style={{ left: pct(r.grx), width: `calc(${pct(r.graphqlGo)} - ${pct(r.grx)})` }}
              />
              <Dot left={pct(r.graphqlGo)} label={`${r.graphqlGo}`} className="size-3.5 bg-fd-muted-foreground/45" />
              <Dot left={pct(r.gophers)} label={`${r.gophers}`} className="size-3.5 border-2 border-grx-400 bg-fd-card" below />
              <Dot
                left={pct(r.grx)}
                label={`${r.grx}`}
                strong
                className="size-4 bg-grx-500 shadow-[0_0_0_5px_rgba(99,102,241,0.2)]"
              />
            </div>
            <p className="font-display text-2xl font-semibold text-accent md:text-right">
              {(r.graphqlGo / r.grx).toFixed(1)}×
            </p>
          </div>
        ))}
      </div>

      <div className="mt-6 hidden grid-cols-[11rem_1fr_6rem] gap-6 md:grid">
        <span />
        <div className="flex justify-between font-mono text-[10px] text-fd-muted-foreground">
          {[0, 100, 200, 300].map((t) => (
            <span key={t}>{t}µs</span>
          ))}
        </div>
        <span className="whitespace-nowrap text-right font-mono text-[10px] text-fd-muted-foreground">vs graphql-go</span>
      </div>
    </div>
  );
}

function Dot({
  left,
  label,
  className,
  strong,
  below,
}: {
  left: string;
  label: string;
  className: string;
  strong?: boolean;
  below?: boolean;
}) {
  return (
    <span className="absolute top-1/2 -translate-x-1/2 -translate-y-1/2" style={{ left }}>
      <span className={`block rounded-full ${className}`} />
      <span
        className={`absolute left-1/2 -translate-x-1/2 whitespace-nowrap font-mono text-[10.5px] ${
          below ? 'top-5' : '-top-5'
        } ${strong ? 'font-semibold text-accent' : 'text-fd-muted-foreground'}`}
      >
        {label}
      </span>
    </span>
  );
}
