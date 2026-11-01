'use client';
import Link from 'next/link';
import { ThemeSwitch, type ThemeSwitchProps } from 'fumadocs-ui/layouts/shared/slots/theme-switch';
import { headerLinks } from '@/lib/shared';

/**
 * Stands in for the header's theme switch so the links sit just before it.
 * The mobile menu renders this slot too; it lists the links itself, so they are hidden there.
 */
export function LinksThenThemeSwitch(props: ThemeSwitchProps) {
  return (
    <>
      <nav className="hidden items-center gap-6 pe-3 text-sm lg:flex">
        {headerLinks.map((l) => (
          <Link key={l.url} href={l.url} className="text-fd-muted-foreground transition-colors hover:text-fd-foreground">
            {l.text}
          </Link>
        ))}
      </nav>
      <ThemeSwitch {...props} />
    </>
  );
}
