import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';
import { BookOpen, Library } from 'lucide-react';
import { GrxMark } from '@/components/grx-mark';
import { LinksThenThemeSwitch } from '@/components/header-links';
import { githubUrl } from './shared';

export function Logo() {
  return (
    <span className="font-display inline-flex items-center gap-2 text-lg font-semibold">
      <GrxMark className="h-7 w-auto" strokeWidth={6} />
      grx
    </span>
  );
}

/**
 * `linksBeforeTheme` (landing page) renders the links on the right of the header, just before
 * the theme switch, and keeps them in the mobile menu. The docs layouts leave it off so the
 * links sit at the top of the sidebar.
 */
export function baseOptions({ linksBeforeTheme = false }: { linksBeforeTheme?: boolean } = {}): BaseLayoutProps {
  return {
    nav: {
      title: <Logo />,
      url: '/',
    },
    githubUrl,
    slots: linksBeforeTheme ? { themeSwitch: LinksThenThemeSwitch } : undefined,
    links: [
      {
        text: 'Docs',
        url: '/getting-started/',
        icon: <BookOpen />,
        active: 'none',
        on: linksBeforeTheme ? 'menu' : 'all',
      },
      {
        text: 'API reference',
        url: '/reference/',
        icon: <Library />,
        active: 'nested-url',
        on: linksBeforeTheme ? 'menu' : 'all',
      },
    ],
  };
}
