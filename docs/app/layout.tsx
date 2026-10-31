import { Archivo, Inter, JetBrains_Mono } from 'next/font/google';
import type { Metadata } from 'next';
import { Provider } from '@/components/provider';
import { appDescription, appName, siteUrl } from '@/lib/shared';
import './global.css';

const inter = Inter({
  subsets: ['latin'],
  variable: '--font-sans',
});

const mono = JetBrains_Mono({
  subsets: ['latin'],
  variable: '--font-mono',
});

const display = Archivo({
  subsets: ['latin'],
  axes: ['wdth'],
  variable: '--font-display',
});

export const metadata: Metadata = {
  metadataBase: new URL(`${siteUrl}/`),
  title: {
    template: `%s · ${appName}`,
    default: `${appName}: GraphQL in Go, where structs are the schema`,
  },
  description: appDescription,
  icons: { icon: '/grx/favicon.svg' },
  openGraph: {
    siteName: appName,
    type: 'website',
  },
};

export default function Layout({ children }: LayoutProps<'/'>) {
  return (
    <html
      lang="en"
      className={`${inter.variable} ${mono.variable} ${display.variable} ${inter.className}`}
      suppressHydrationWarning
    >
      <body className="flex flex-col min-h-screen">
        <Provider>{children}</Provider>
      </body>
    </html>
  );
}
