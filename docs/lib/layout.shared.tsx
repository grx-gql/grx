import Link from 'next/link';
import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';
import { BookOpen, Library } from 'lucide-react';
import { GrxMark } from '@/components/grx-mark';
import { githubUrl } from './shared';

export function Logo() {
  return (
    <span className="font-display inline-flex items-center gap-2 text-lg font-semibold">
      <GrxMark className="h-7 w-auto" strokeWidth={6} />
      grx
    </span>
  );
}

const navLinks = [
  { text: 'Docs', url: '/getting-started/', icon: <BookOpen />, active: 'none' as const },
  { text: 'API reference', url: '/reference/', icon: <Library />, active: 'nested-url' as const },
];

function CenteredLinks() {
  return (
    <nav className="absolute left-1/2 hidden -translate-x-1/2 items-center gap-7 text-sm lg:flex">
      {navLinks.map((l) => (
        <Link key={l.url} href={l.url} className="text-fd-muted-foreground transition-colors hover:text-fd-foreground">
          {l.text}
        </Link>
      ))}
    </nav>
  );
}

/**
 * `centerLinks` (landing page) renders the links centered in the header on desktop and keeps
 * them in the mobile menu. The docs layouts leave it off so the links sit atop the sidebar.
 */
export function baseOptions({ centerLinks = false }: { centerLinks?: boolean } = {}): BaseLayoutProps {
  return {
    nav: {
      title: <Logo />,
      url: '/',
      children: centerLinks ? <CenteredLinks /> : undefined,
    },
    githubUrl,
    links: navLinks.map((l) => ({ ...l, on: centerLinks ? ('menu' as const) : ('all' as const) })),
  };
}
