'use client';
import { useEffect, useMemo, useRef, useState } from 'react';

type Line = { kind: 'cmd' | 'out' | 'ok' | 'json'; text: string };

const CHAR_MS = 18;
const OUTPUT_PAUSE_MS = 260;

export function TerminalDemo({ version }: { version: string }) {
  const lines = useMemo<Line[]>(() => [
    { kind: 'cmd', text: 'go mod init example.com/api' },
    { kind: 'cmd', text: 'go get github.com/grx-gql/grx@latest' },
    { kind: 'out', text: `go: added github.com/grx-gql/grx ${version}` },
    { kind: 'cmd', text: 'go run . &' },
    { kind: 'ok', text: 'GraphiQL: http://localhost:4000/' },
    {
      kind: 'cmd',
      text: `curl -s localhost:4000/graphql -H 'content-type: application/json' -d '{"query":"{ user(id: \\"1\\") { name } }"}'`,
    },
    { kind: 'json', text: '{"data":{"user":{"name":"Ada Lovelace"}}}' },
  ], [version]);

  const ref = useRef<HTMLDivElement>(null);
  const [lineIndex, setLineIndex] = useState(0);
  const [chars, setChars] = useState(0);
  const [started, setStarted] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      setLineIndex(lines.length);
      return;
    }
    if (el.getBoundingClientRect().top < window.innerHeight * 0.8) {
      setStarted(true);
      return;
    }
    const io = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setStarted(true);
          io.disconnect();
        }
      },
      { threshold: 0.4 },
    );
    io.observe(el);
    return () => io.disconnect();
  }, [lines.length]);

  useEffect(() => {
    if (!started || lineIndex >= lines.length) return;
    const line = lines[lineIndex];
    if (line.kind === 'cmd' && chars < line.text.length) {
      const id = setTimeout(() => setChars((c) => c + 1), CHAR_MS);
      return () => clearTimeout(id);
    }
    const id = setTimeout(
      () => {
        setLineIndex((i) => i + 1);
        setChars(0);
      },
      line.kind === 'cmd' ? OUTPUT_PAUSE_MS : 140,
    );
    return () => clearTimeout(id);
  }, [started, lineIndex, chars, lines]);

  return (
    <div
      ref={ref}
      className="overflow-hidden rounded-lg border border-grx-300/15 bg-night font-mono text-[12.5px] leading-6 text-white/85 shadow-[0_24px_60px_-28px_rgba(67,56,202,0.5)]"
    >
      <div className="flex items-center gap-2 border-b border-white/10 px-4 py-2.5">
        <span className="size-2.5 rounded-full bg-white/15" />
        <span className="size-2.5 rounded-full bg-white/15" />
        <span className="size-2.5 rounded-full bg-white/15" />
        <span className="ms-2 text-[11px] text-white/40">~/api — zsh</span>
      </div>
      <div className="min-h-[300px] px-4 py-4 [overflow-wrap:anywhere]">
        {lines.slice(0, lineIndex + 1).map((line, i) => {
          if (i > lineIndex) return null;
          const typing = i === lineIndex;
          if (line.kind === 'cmd') {
            const text = typing ? line.text.slice(0, chars) : line.text;
            return (
              <p key={i}>
                <span className="text-grx-300">❯ </span>
                {text}
                {typing && lineIndex < lines.length && <span className="grx-caret ms-px inline-block h-4 w-2 translate-y-0.5 bg-white/70" />}
              </p>
            );
          }
          if (typing) return null;
          return (
            <p
              key={i}
              className={
                line.kind === 'ok' ? 'text-grx-200' : line.kind === 'json' ? 'text-[#a5d6ff]' : 'text-white/45'
              }
            >
              {line.text}
            </p>
          );
        })}
        {lineIndex >= lines.length && (
          <p>
            <span className="text-grx-300">❯ </span>
            <span className="grx-caret inline-block h-4 w-2 translate-y-0.5 bg-white/70" />
          </p>
        )}
      </div>
    </div>
  );
}
