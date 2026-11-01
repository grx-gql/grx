'use client';
import Link from 'next/link';
import { Boxes, FileLock2, Layers, Plug, Radio, ShieldCheck } from 'lucide-react';
import { type ReactNode, useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { cn } from '@/lib/cn';

type Option = { call: string; arg: string; icon: ReactNode; title: string; body: string; href: string };

const options: Option[] = [
  {
    call: 'WithSchema',
    arg: 'graph.Schema()',
    icon: <Boxes />,
    title: 'Schema from structs',
    body: 'Exported structs become types, methods become fields. No SDL to keep in sync.',
    href: '/concepts/schema-basics/',
  },
  {
    call: 'WithTransports',
    arg: 'websocket.New(), sse.New()',
    icon: <Radio />,
    title: 'Realtime subscriptions',
    body: 'graphql-transport-ws and SSE, fanned out by in-memory or Redis pub/sub.',
    href: '/guides/subscriptions/',
  },
  {
    call: 'WithPlugins',
    arg: 'logger',
    icon: <Plug />,
    title: 'Hooks at every stage',
    body: 'Trace, cache and measure parse, validate and execute. Structured slog logging built in.',
    href: '/concepts/plugins/',
  },
  {
    call: 'WithPersistedQueries',
    arg: 'trusted',
    icon: <FileLock2 />,
    title: 'Persisted queries',
    body: 'Automatic persisted queries and trusted documents for public APIs.',
    href: '/guides/persisted-queries/',
  },
  {
    call: 'WithFieldAuthorizer',
    arg: 'requireViewer',
    icon: <ShieldCheck />,
    title: 'Field-level auth',
    body: 'Decide per field, with the caller’s identity already in the request context.',
    href: '/guides/auth/',
  },
  {
    call: 'WithMaxSelectionDepth',
    arg: '12',
    icon: <Layers />,
    title: 'Guard rails',
    body: 'Depth, body size and timeouts stop abusive queries before they reach resolvers.',
    href: '/guides/request-limits/',
  },
];

type Edge = { d: string; x1: number; y1: number; x2: number; y2: number };

const rowStart = ['xl:row-start-1', 'xl:row-start-2', 'xl:row-start-3'];

function OptionCard({
  o,
  i,
  active,
  onActivate,
  cardRef,
}: {
  o: Option;
  i: number;
  active: boolean;
  onActivate: (i: number) => void;
  cardRef: (el: HTMLAnchorElement | null) => void;
}) {
  return (
    <Link
      ref={cardRef}
      href={o.href}
      onMouseEnter={() => onActivate(i)}
      onFocus={() => onActivate(i)}
      className={cn(
        'relative block h-full rounded-md border bg-fd-card p-5 transition duration-300',
        active ? 'border-grx-400/70 shadow-lg shadow-grx-500/10' : 'border-fd-border',
      )}
    >
      <div className="flex items-center gap-3">
        <span
          className={cn(
            'flex size-9 shrink-0 items-center justify-center rounded-full border transition-colors [&_svg]:size-4',
            active ? 'border-grx-500 bg-grx-500 text-white' : 'border-fd-border text-fd-muted-foreground',
          )}
        >
          {o.icon}
        </span>
        <div className="min-w-0">
          <p className="font-semibold">{o.title}</p>
          <p className="truncate font-mono text-[11px] text-fd-muted-foreground">grx.{o.call}</p>
        </div>
      </div>
      <p className="mt-3 text-sm leading-relaxed text-fd-muted-foreground">{o.body}</p>
    </Link>
  );
}

export function OptionsGraph() {
  const [active, setActive] = useState(0);
  const [touched, setTouched] = useState(false);
  const [edges, setEdges] = useState<Edge[]>([]);
  const wrap = useRef<HTMLDivElement>(null);
  const center = useRef<HTMLDivElement>(null);
  const cards = useRef<(HTMLAnchorElement | null)[]>([]);
  const lines = useRef<(HTMLSpanElement | null)[]>([]);

  const measure = useCallback(() => {
    const w = wrap.current?.getBoundingClientRect();
    const c = center.current?.getBoundingClientRect();
    if (!w || !c || window.innerWidth < 1280) {
      setEdges([]);
      return;
    }
    setEdges(
      options.map((_, i) => {
        const card = cards.current[i]!.getBoundingClientRect();
        const line = lines.current[i]!.getBoundingClientRect();
        const left = i < 3;
        const x1 = (left ? card.right : card.left) - w.left;
        const y1 = card.top + card.height / 2 - w.top;
        const x2 = (left ? c.left : c.right) - w.left;
        const y2 = line.top + line.height / 2 - w.top;
        const mx = (x1 + x2) / 2;
        return { d: `M ${x1} ${y1} C ${mx} ${y1}, ${mx} ${y2}, ${x2} ${y2}`, x1, y1, x2, y2 };
      }),
    );
  }, []);

  useLayoutEffect(() => {
    measure();
    const ro = new ResizeObserver(measure);
    if (wrap.current) ro.observe(wrap.current);
    return () => ro.disconnect();
  }, [measure]);

  useEffect(() => {
    if (touched) return;
    const id = setInterval(() => setActive((a) => (a + 1) % options.length), 2400);
    return () => clearInterval(id);
  }, [touched]);

  const activate = (i: number) => {
    setTouched(true);
    setActive(i);
  };

  return (
    <div ref={wrap} className="relative">
      <svg aria-hidden className="pointer-events-none absolute inset-0 hidden h-full w-full overflow-visible xl:block">
        {edges.map((e, i) => (
          <g key={i}>
            <path d={e.d} fill="none" stroke="var(--color-fd-border)" strokeWidth="1.5" />
            {i === active && <path d={e.d} fill="none" stroke="#6366f1" strokeWidth="1.8" className="grx-flow" />}
            <circle cx={e.x1} cy={e.y1} r="3.5" fill={i === active ? '#6366f1' : 'var(--color-fd-border)'} />
            <circle cx={e.x2} cy={e.y2} r="3.5" fill={i === active ? '#6366f1' : 'var(--color-fd-border)'} />
          </g>
        ))}
      </svg>

      <div className="relative grid gap-4 md:grid-cols-2 xl:grid-cols-[1fr_minmax(0,27.5rem)_1fr] xl:gap-x-16">
        <div
          ref={center}
          className="overflow-hidden rounded-lg border border-grx-300/15 bg-night text-white shadow-[0_24px_60px_-24px_rgba(67,56,202,0.5)] md:col-span-2 md:mb-2 xl:col-span-1 xl:col-start-2 xl:row-span-3 xl:row-start-1 xl:mb-0 xl:self-center"
        >
          <div className="border-b border-white/10 px-5 py-3 font-mono text-[11px] text-white/45">main.go</div>
          <pre className="grx-scroll overflow-x-auto px-5 py-4 font-mono text-[12.5px] leading-7">
            <span className="text-white/85">
              srv, err := grx.<span className="text-[#d2a8ff]">NewServer</span>(
            </span>
            {'\n'}
            {options.map((o, i) => (
              <span
                key={o.call}
                ref={(el) => {
                  lines.current[i] = el;
                }}
                onMouseEnter={() => activate(i)}
                className={cn(
                  '-mx-5 block cursor-default border-l-2 px-5 transition-colors',
                  i === active ? 'border-grx-400 bg-grx-500/15' : 'border-transparent',
                )}
              >
                {'    '}grx.<span className="text-[#d2a8ff]">{o.call}</span>(
                <span className="text-[#a5d6ff]">{o.arg}</span>),
              </span>
            ))}
            <span className="text-white/85">)</span>
          </pre>
        </div>

        {options.map((o, i) => (
          <div key={o.call} className={cn(i < 3 ? 'xl:col-start-1' : 'xl:col-start-3', rowStart[i % 3])}>
            <OptionCard
              o={o}
              i={i}
              active={i === active}
              onActivate={activate}
              cardRef={(el) => {
                cards.current[i] = el;
              }}
            />
          </div>
        ))}
      </div>
    </div>
  );
}
