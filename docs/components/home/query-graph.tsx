'use client';
import Link from 'next/link';
import { RotateCcw } from 'lucide-react';
import { useEffect, useId, useRef, useState } from 'react';
import { cn } from '@/lib/cn';

type Kind = 'root' | 'mid' | 'leaf';
type GraphNode = {
  id: string;
  parent?: string;
  label: string;
  value?: string;
  x: number;
  y: number;
  kind: Kind;
};

// The ParameterizedNested benchmark query (benchmark/scenarios.go), resolved depth-first.
const nodes: GraphNode[] = [
  { id: 'q', label: 'query', x: 58, y: 170, kind: 'root' },
  { id: 'post', parent: 'q', label: 'asset: post', x: 178, y: 170, kind: 'mid' },
  { id: 'id', parent: 'post', label: 'id', value: '"post_1"', x: 318, y: 50, kind: 'leaf' },
  { id: 'title', parent: 'post', label: 'title', value: '"Hello, grx"', x: 318, y: 113, kind: 'leaf' },
  { id: 'body', parent: 'post', label: 'body', value: '"Structs in, schema out."', x: 318, y: 176, kind: 'leaf' },
  { id: 'author', parent: 'post', label: 'author', x: 318, y: 262, kind: 'mid' },
  { id: 'aid', parent: 'author', label: 'id', value: '"user_1"', x: 458, y: 218, kind: 'leaf' },
  { id: 'aname', parent: 'author', label: 'name', value: '"Ada Lovelace"', x: 458, y: 264, kind: 'leaf' },
  { id: 'aemail', parent: 'author', label: 'email', value: '"ada@example.com"', x: 458, y: 310, kind: 'leaf' },
];

const radius: Record<Kind, number> = { root: 22, mid: 13, leaf: 8 };

// Outline used instead of the SVG on phones, where the graph would be too small to read.
const treePrefix: Record<string, string> = {
  q: '',
  post: '└─ ',
  id: '   ├─ ',
  title: '   ├─ ',
  body: '   ├─ ',
  author: '   └─ ',
  aid: '      ├─ ',
  aname: '      ├─ ',
  aemail: '      └─ ',
};
const byId = Object.fromEntries(nodes.map((n) => [n.id, n]));

function edgePath(n: GraphNode) {
  const p = byId[n.parent!];
  const x1 = p.x + radius[p.kind];
  const x2 = n.x - radius[n.kind];
  const mx = (x1 + x2) / 2;
  return `M ${x1} ${p.y} C ${mx} ${p.y}, ${mx} ${n.y}, ${x2} ${n.y}`;
}

const responseParts = [
  '{"data":{"asset":{',
  '"id":"post_1",',
  '"title":"Hello, grx",',
  '"body":"Structs in, schema out.",',
  '"author":{',
  '"id":"user_1",',
  '"name":"Ada Lovelace",',
  '"email":"ada@example.com"}}}}',
];
// Index of the step after which each response part is visible.
const responseAt = [1, 2, 3, 4, 5, 6, 7, 8];

const STEP_MS = 420;
const HOLD_MS = 5200;

export function QueryGraph() {
  const uid = useId().replace(/:/g, '');
  const ref = useRef<HTMLDivElement>(null);
  const [step, setStep] = useState(-1);
  const [visible, setVisible] = useState(false);
  const [run, setRun] = useState(0);
  const done = step >= nodes.length - 1;

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      setStep(nodes.length - 1);
      return;
    }
    const rect = el.getBoundingClientRect();
    setVisible(rect.top < window.innerHeight && rect.bottom > 0);
    const io = new IntersectionObserver(([entry]) => setVisible(entry.isIntersecting));
    io.observe(el);
    return () => io.disconnect();
  }, []);

  useEffect(() => {
    if (!visible) return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    let i = -1;
    let timer: ReturnType<typeof setTimeout>;
    const next = () => {
      i += 1;
      setStep(i);
      if (i < nodes.length - 1) {
        timer = setTimeout(next, STEP_MS);
      } else {
        timer = setTimeout(() => {
          i = -1;
          setStep(-1);
          timer = setTimeout(next, 500);
        }, HOLD_MS);
      }
    };
    setStep(-1);
    timer = setTimeout(next, 450);
    return () => clearTimeout(timer);
  }, [visible, run]);

  const resolvedLeaves = nodes.filter((n, i) => n.kind === 'leaf' && i <= step).length;
  const totalLeaves = nodes.filter((n) => n.kind === 'leaf').length;

  return (
    <div
      ref={ref}
      className="overflow-hidden rounded-lg border border-grx-300/15 bg-night text-white shadow-[0_30px_80px_-20px_rgba(67,56,202,0.45)]"
    >
      <div className="flex items-center gap-3 border-b border-white/10 px-4 py-3 font-mono text-[11.5px]">
        <span className="rounded bg-grx-500/20 px-1.5 py-0.5 font-semibold text-grx-200">POST</span>
        <span className="min-w-0 truncate text-white/60">
          /graphql <span className="text-white/30">·</span>{' '}
          <span className="text-white/85">DetailScreen</span>($pid: <span className="text-grx-200">"post_1"</span>)
        </span>
        <button
          type="button"
          onClick={() => setRun((r) => r + 1)}
          className="ms-auto inline-flex shrink-0 items-center gap-1.5 rounded-md px-2 py-1 text-white/50 transition hover:bg-white/5 hover:text-white"
          aria-label="Replay the query"
        >
          <RotateCcw className="size-3.5" />
          <span className="hidden sm:inline">replay</span>
        </button>
      </div>

      <div className="grx-night-grid">
        <ol aria-hidden className="space-y-1 px-4 py-5 font-mono text-[12.5px] leading-6 sm:hidden">
          {nodes.map((n, i) => {
            const on = i <= step;
            return (
              <li key={n.id} className="flex min-w-0 items-center whitespace-pre">
                <span className="text-grx-300/35">{treePrefix[n.id]}</span>
                <span
                  className={cn(
                    'me-2 inline-block shrink-0 rounded-full border transition-colors duration-300',
                    n.kind === 'root' ? 'size-3' : n.kind === 'mid' ? 'size-2.5' : 'size-2',
                    on ? 'border-grx-300 bg-grx-400' : 'border-grx-300/40 bg-transparent',
                    i === step && 'shadow-[0_0_0_4px_rgba(129,140,248,0.3)]',
                  )}
                />
                <span className={cn('transition-colors duration-300', on ? 'text-grx-50' : 'text-grx-300/45')}>
                  {n.kind === 'root' ? '{ } query' : n.label}
                </span>
                {n.value && (
                  <span
                    className="ms-2 min-w-0 truncate text-grx-300 transition-opacity duration-300"
                    style={{ opacity: on ? 0.9 : 0 }}
                  >
                    {n.value}
                  </span>
                )}
              </li>
            );
          })}
        </ol>
        <svg viewBox="0 0 600 340" className="hidden h-auto w-full sm:block" role="img" aria-label="The DetailScreen query resolving field by field">
          <defs>
            <linearGradient id={`${uid}e`} x1="0" y1="0" x2="1" y2="0">
              <stop offset="0%" stopColor="#a5b4fc" stopOpacity="0.6" />
              <stop offset="100%" stopColor="#818cf8" />
            </linearGradient>
            <linearGradient id={`${uid}r`} x1="0" y1="0" x2="1" y2="1">
              <stop offset="0%" stopColor="#818cf8" />
              <stop offset="100%" stopColor="#4338ca" />
            </linearGradient>
            <linearGradient id={`${uid}m`} x1="0" y1="0" x2="1" y2="1">
              <stop offset="0%" stopColor="#a5b4fc" />
              <stop offset="100%" stopColor="#6366f1" />
            </linearGradient>
          </defs>

          <g fill="none" strokeLinecap="round">
            {nodes.slice(1).map((n) => (
              <path key={`b-${n.id}`} d={edgePath(n)} stroke="rgba(165,180,252,0.16)" strokeWidth="1.4" />
            ))}
            {nodes.slice(1).map((n) => {
              const i = nodes.indexOf(n);
              return (
                <path
                  key={`a-${n.id}-${run}`}
                  d={edgePath(n)}
                  pathLength={1}
                  className="grx-edge"
                  data-on={i <= step}
                  stroke={`url(#${uid}e)`}
                  strokeWidth="1.8"
                />
              );
            })}
          </g>

          {nodes.map((n, i) => {
            const on = i <= step;
            const r = radius[n.kind];
            const fill = n.kind === 'root' ? `url(#${uid}r)` : n.kind === 'mid' ? `url(#${uid}m)` : '#c7d2fe';
            const labelAbove = n.kind === 'mid';
            return (
              <g key={n.id}>
                {i === step && (
                  <circle key={`ping-${step}-${run}`} cx={n.x} cy={n.y} r={r} fill="none" stroke="#a5b4fc" strokeWidth="1.5" className="grx-ping" />
                )}
                <circle cx={n.x} cy={n.y} r={r} fill="#0c0c1a" stroke="rgba(165,180,252,0.35)" strokeWidth="1.2" />
                <circle
                  cx={n.x}
                  cy={n.y}
                  r={r}
                  fill={fill}
                  style={{ opacity: on ? 1 : 0, transition: 'opacity 0.3s ease' }}
                />
                {n.kind === 'root' && (
                  <>
                    <circle cx={n.x} cy={n.y} r={r} fill="none" stroke="#c7d2fe" strokeOpacity="0.4" strokeWidth="1.5" />
                    <text x={n.x} y={n.y + 5} textAnchor="middle" fontFamily="Inter, system-ui, sans-serif" fontWeight="700" fontSize="13" fill="#fff">
                      {'{ }'}
                    </text>
                    <text x={n.x} y={n.y + 42} textAnchor="middle" className="font-mono" fontSize="11" fill="rgba(199,210,254,0.55)">
                      query
                    </text>
                  </>
                )}
                {n.kind !== 'root' && (
                  <text
                    x={labelAbove ? n.x : n.x + r + 8}
                    y={labelAbove ? n.y - r - 9 : n.y - 3}
                    textAnchor={labelAbove ? 'middle' : 'start'}
                    className="font-mono"
                    fontSize="12"
                    fill={on ? '#e0e7ff' : 'rgba(165,180,252,0.45)'}
                    style={{ transition: 'fill 0.3s ease' }}
                  >
                    {n.label}
                  </text>
                )}
                {n.value && (
                  <text
                    x={n.x + r + 8}
                    y={n.y + 12}
                    className="font-mono"
                    fontSize="11"
                    fill="#a5b4fc"
                    style={{ opacity: on ? 0.9 : 0, transition: 'opacity 0.35s ease 0.12s' }}
                  >
                    {n.value}
                  </text>
                )}
              </g>
            );
          })}
        </svg>
      </div>

      <div className="grid border-t border-white/10 sm:grid-cols-[1fr_14.5rem]">
        <div className="min-w-0 px-4 py-3.5">
          <p className="font-mono text-[10px] uppercase tracking-[0.14em] text-white/35">Response</p>
          {/* The complete response sits invisibly underneath so the box never changes height. */}
          <div className="mt-1.5 grid font-mono text-[11.5px] leading-relaxed [overflow-wrap:anywhere]">
            <p aria-hidden className="invisible [grid-area:1/1]">
              {responseParts.join('')}
              <span className="ms-px inline-block h-3 w-1.5" />
            </p>
            <p className="text-grx-100/85 [grid-area:1/1]">
              {responseParts.filter((_, i) => responseAt[i] <= step).join('')}
              {!done && <span className="grx-caret ms-px inline-block h-3 w-1.5 translate-y-0.5 bg-grx-300/70" />}
            </p>
          </div>
        </div>
        <div className="grid border-t border-white/10 px-4 py-3.5 sm:border-l sm:border-t-0">
          <div
            key={done ? `done-${run}` : 'pending'}
            aria-hidden={!done}
            inert={!done}
            className={cn('[grid-area:1/1]', done ? 'grx-fade-up' : 'invisible')}
          >
            <p className="font-display text-2xl font-semibold text-white">
              9.49<span className="ms-0.5 text-base text-white/60">µs</span>
            </p>
            <p className="mt-0.5 text-[11.5px] leading-snug text-white/55">
              per op on grx: <span className="text-grx-200">2.0×</span> faster than graph-gophers,{' '}
              <span className="text-grx-200">12.7×</span> faster than graphql-go
            </p>
            <Link href="/benchmarks/" className="mt-1.5 inline-block font-mono text-[11px] text-grx-300 hover:text-white">
              same query, benchmarked →
            </Link>
          </div>
          <div aria-hidden={done} className={cn('[grid-area:1/1]', done && 'invisible')}>
            <p className="font-mono text-[10px] uppercase tracking-[0.14em] text-white/35">Resolving</p>
            <p className="mt-1.5 font-mono text-sm text-white/80">
              {resolvedLeaves}/{totalLeaves} <span className="text-white/40">fields</span>
            </p>
            <div className="mt-2 h-1 overflow-hidden rounded-full bg-white/10">
              <div
                className={cn('h-full rounded-full bg-grx-400 transition-[width] duration-300')}
                style={{ width: `${(resolvedLeaves / totalLeaves) * 100}%` }}
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
