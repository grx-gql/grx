import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';
import { BookOpen, FlaskConical, Gauge, Library, MessagesSquare } from 'lucide-react';
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

export function baseOptions(): BaseLayoutProps {
  return {
    nav: {
      title: <Logo />,
      url: '/',
    },
    githubUrl,
    links: [
      {
        text: 'Docs',
        url: '/getting-started/',
        icon: <BookOpen />,
        active: 'none',
      },
      {
        text: 'API reference',
        url: '/reference/',
        icon: <Library />,
        active: 'nested-url',
      },
      {
        text: 'Examples',
        url: '/examples/',
        icon: <FlaskConical />,
      },
      {
        text: 'Benchmarks',
        url: '/benchmarks/',
        icon: <Gauge />,
      },
      {
        text: 'Discussions',
        url: `${githubUrl}/discussions`,
        icon: <MessagesSquare />,
        external: true,
      },
    ],
  };
}
