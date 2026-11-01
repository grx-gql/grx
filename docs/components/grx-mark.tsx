import { useId } from 'react';

// The original grx execution-graph mark (docs/public/hero.svg), cropped to the graph.
const edges = [
  'M 110 210 C 160 210, 170 110, 230 100',
  'M 110 210 C 170 210, 180 210, 230 210',
  'M 110 210 C 160 210, 170 310, 230 320',
  'M 230 100 C 280 100, 300 60, 380 60',
  'M 230 100 C 280 100, 300 130, 380 140',
  'M 230 210 C 290 210, 310 190, 380 200',
  'M 230 210 C 290 210, 310 240, 380 260',
  'M 230 320 C 280 320, 300 290, 380 300',
  'M 230 320 C 280 320, 300 350, 380 360',
];
const leaves = [60, 140, 200, 260, 300, 360];
const mids = [100, 210, 320];

export function GrxMark({ className, strokeWidth = 1.6 }: { className?: string; strokeWidth?: number }) {
  const id = useId().replace(/:/g, '');
  return (
    <svg viewBox="78 44 316 332" className={className} aria-hidden>
      <defs>
        <linearGradient id={`${id}e`} x1="0" y1="0" x2="1" y2="0">
          <stop offset="0%" stopColor="#a5b4fc" stopOpacity="0.55" />
          <stop offset="100%" stopColor="#6366f1" stopOpacity="0.95" />
        </linearGradient>
        <linearGradient id={`${id}r`} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#818cf8" />
          <stop offset="100%" stopColor="#4338ca" />
        </linearGradient>
        <linearGradient id={`${id}m`} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#a5b4fc" />
          <stop offset="100%" stopColor="#6366f1" />
        </linearGradient>
      </defs>
      <g stroke={`url(#${id}e)`} strokeWidth={strokeWidth} fill="none" strokeLinecap="round">
        {edges.map((d) => (
          <path key={d} d={d} />
        ))}
      </g>
      {leaves.map((y) => (
        <circle key={y} cx="380" cy={y} r="9" fill="#c7d2fe" />
      ))}
      {mids.map((y) => (
        <circle key={y} cx="230" cy={y} r="14" fill={`url(#${id}m)`} />
      ))}
      <circle cx="110" cy="210" r="26" fill={`url(#${id}r)`} />
      <circle cx="110" cy="210" r="26" fill="none" stroke="#c7d2fe" strokeOpacity="0.4" strokeWidth="1.5" />
      <text
        x="110"
        y="216"
        textAnchor="middle"
        fontFamily="Inter, system-ui, sans-serif"
        fontWeight="700"
        fontSize="14"
        fill="#ffffff"
      >
        {'{ }'}
      </text>
    </svg>
  );
}
