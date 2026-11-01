'use client';
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { cn } from '@/lib/cn';

type Line = { text: string; ids: string[] };

const goLines: Line[] = [
  { text: 'type User struct {', ids: ['u'] },
  { text: '\tID   string  `gql:"id,nonNull"`', ids: ['u-id'] },
  { text: '\tName string  `gql:"name,nonNull"`', ids: ['u-name'] },
  { text: '\tBio  *string `gql:"bio"`', ids: ['u-bio'] },
  { text: '}', ids: ['u'] },
  { text: '', ids: [] },
  { text: 'type UserArgs struct {', ids: ['args'] },
  { text: '\tID string `gql:"id,nonNull"`', ids: ['args'] },
  { text: '}', ids: ['args'] },
  { text: '', ids: [] },
  { text: 'type Query struct{}', ids: ['q'] },
  { text: '', ids: [] },
  { text: 'func (Query) User(ctx context.Context, args UserArgs) (*User, error)', ids: ['q-user'] },
  { text: '', ids: [] },
  { text: 'type Subscription struct{}', ids: ['s'] },
  { text: '', ids: [] },
  { text: 'func (Subscription) UserUpdated(ctx context.Context) (<-chan *User, error)', ids: ['s-upd'] },
];

const sdlLines: Line[] = [
  { text: 'type User {', ids: ['u'] },
  { text: '  id: String!', ids: ['u-id'] },
  { text: '  name: String!', ids: ['u-name'] },
  { text: '  bio: String', ids: ['u-bio'] },
  { text: '}', ids: ['u'] },
  { text: '', ids: [] },
  { text: 'type Query {', ids: ['q'] },
  { text: '  user(id: String!): User', ids: ['q-user', 'args'] },
  { text: '}', ids: ['q'] },
  { text: '', ids: [] },
  { text: 'type Subscription {', ids: ['s'] },
  { text: '  userUpdated: User', ids: ['s-upd'] },
  { text: '}', ids: ['s'] },
];

const notes: Record<string, string> = {
  u: 'Any struct a resolver returns becomes a GraphQL object type.',
  'u-id': 'The gql tag names the field; nonNull adds the `!`.',
  'u-name': 'Exported struct fields become fields on the object type.',
  'u-bio': 'Without nonNull, a field stays nullable.',
  args: 'A single args struct becomes the field’s arguments.',
  q: 'Query is the one required root type.',
  'q-user': 'Exported methods on Query become fields: User → user.',
  s: 'Subscription is an optional root, served over WebSocket or SSE.',
  's-upd': 'Return a receive-only channel and every value is pushed to the client.',
};

const tour = ['u', 'u-id', 'u-bio', 'args', 'q-user', 's-upd'];

const goKeywords = new Set(['type', 'struct', 'func', 'chan', 'return']);
const builtins = new Set(['string', 'error', 'String', 'Int', 'Boolean', 'Float', 'ID']);

function Tokens({ text, sdl }: { text: string; sdl?: boolean }) {
  const parts = text.match(/(`[^`]*`|"[^"]*"|[A-Za-z_]\w*|\s+|.)/g) ?? [];
  return (
    <>
      {parts.map((p, i) => {
        let color = 'text-[#e6edf3]';
        if (p.startsWith('`') || p.startsWith('"')) color = 'text-[#a5d6ff]';
        else if (sdl ? p === 'type' : goKeywords.has(p)) color = 'text-[#ff7b72]';
        else if (builtins.has(p)) color = 'text-[#79c0ff]';
        else if (/^[A-Z]/.test(p)) color = 'text-[#d2a8ff]';
        else if (sdl && /^\w+$/.test(p)) color = 'text-[#ffa657]';
        return (
          <span key={i} className={color}>
            {p.replace(/\t/g, '    ')}
          </span>
        );
      })}
    </>
  );
}

function Pane({
  title,
  badge,
  lines,
  active,
  onPick,
  sdl,
}: {
  title: string;
  badge: string;
  lines: Line[];
  active: string;
  onPick: (id: string) => void;
  sdl?: boolean;
}) {
  return (
    <div
      data-pane={sdl ? 'sdl' : 'go'}
      className="min-w-0 overflow-hidden rounded-lg border border-grx-300/15 bg-night shadow-[0_24px_60px_-28px_rgba(67,56,202,0.5)]"
    >
      <div className="flex items-center gap-2 border-b border-white/10 px-4 py-2.5 font-mono text-xs text-white/70">
        <span
          className={cn(
            'rounded px-1 py-px text-[10px] font-semibold uppercase',
            sdl ? 'bg-pink-500/15 text-pink-300' : 'bg-grx-500/20 text-grx-200',
          )}
        >
          {badge}
        </span>
        {title}
      </div>
      <pre className="grx-scroll overflow-x-auto py-3 font-mono text-[12.5px] leading-6">
        {lines.map((line, i) => {
          const on = line.ids.includes(active);
          return (
            <div
              key={i}
              data-on={on}
              onMouseEnter={() => line.ids[0] && onPick(line.ids[0])}
              onClick={() => line.ids[0] && onPick(line.ids[0])}
              className={cn(
                'min-h-6 border-l-2 px-4 transition-colors duration-200',
                line.ids.length > 0 && 'cursor-pointer',
                on ? 'border-grx-400 bg-grx-500/15' : 'border-transparent',
              )}
            >
              <Tokens text={line.text} sdl={sdl} />
            </div>
          );
        })}
      </pre>
    </div>
  );
}

export function SchemaMapper() {
  const [active, setActive] = useState('q-user');
  const [touched, setTouched] = useState(false);

  useEffect(() => {
    if (touched) return;
    const id = setInterval(() => {
      setActive((cur) => tour[(tour.indexOf(cur) + 1) % tour.length]);
    }, 2600);
    return () => clearInterval(id);
  }, [touched]);

  function pick(id: string) {
    setTouched(true);
    setActive(id);
  }

  const grid = useRef<HTMLDivElement>(null);
  const [link, setLink] = useState('');

  const measure = useCallback(() => {
    const g = grid.current;
    if (!g || window.innerWidth < 1280) {
      setLink('');
      return;
    }
    const svg = g.querySelector('svg')!.getBoundingClientRect();
    const from = g.querySelector('[data-pane="go"] [data-on="true"]')?.getBoundingClientRect();
    const to = g.querySelector('[data-pane="sdl"] [data-on="true"]')?.getBoundingClientRect();
    if (!from || !to) {
      setLink('');
      return;
    }
    const y1 = from.top + from.height / 2 - svg.top;
    const y2 = to.top + to.height / 2 - svg.top;
    const w = svg.width;
    setLink(`M 0 ${y1} C ${w / 2} ${y1}, ${w / 2} ${y2}, ${w} ${y2}`);
  }, []);

  useLayoutEffect(() => {
    measure();
  }, [active, measure]);

  useEffect(() => {
    window.addEventListener('resize', measure);
    return () => window.removeEventListener('resize', measure);
  }, [measure]);

  return (
    <div>
      <div ref={grid} className="relative grid gap-4 xl:grid-cols-[1.35fr_5rem_1fr] xl:gap-0">
        <Pane title="graph/user.go" badge="go" lines={goLines} active={active} onPick={pick} />
        <svg aria-hidden className="pointer-events-none hidden h-full w-full overflow-visible xl:block">
          {link && (
            <>
              <path d={link} fill="none" stroke="var(--color-fd-border)" strokeWidth="1.5" />
              <path d={link} fill="none" stroke="#6366f1" strokeWidth="1.8" className="grx-flow" />
            </>
          )}
        </svg>
        <Pane title="schema (generated at startup)" badge="gql" lines={sdlLines} active={active} onPick={pick} sdl />
      </div>
      <p
        aria-live="polite"
        className="mx-auto mt-6 flex max-w-2xl items-center justify-center gap-2 text-center text-sm text-fd-muted-foreground"
      >
        <span className="grx-pulse inline-block size-2 shrink-0 rounded-full bg-grx-500" />
        {notes[active]}
      </p>
    </div>
  );
}
