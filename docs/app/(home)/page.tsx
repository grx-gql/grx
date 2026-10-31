import Link from 'next/link';
import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { ArrowRight, ArrowUpRight } from 'lucide-react';
import { GrxMark } from '@/components/grx-mark';
import { InstallCommand } from '@/components/home/install-command';
import { OptionsGraph } from '@/components/home/options-graph';
import { QueryGraph } from '@/components/home/query-graph';
import { SchemaMapper } from '@/components/home/schema-mapper';
import { SpeedPlot } from '@/components/home/speed-plot';
import { TerminalDemo } from '@/components/home/terminal-demo';
import { appDescription, githubUrl, siteUrl } from '@/lib/shared';
import { latestRelease } from '@/lib/version';
import { cn } from '@/lib/cn';

const title = 'grx: GraphQL, resolved in plain Go';

export const metadata: Metadata = {
  title: { absolute: title },
  description: appDescription,
  alternates: { canonical: `${siteUrl}/` },
  openGraph: { title, description: appDescription, url: `${siteUrl}/`, images: `${siteUrl}/hero.png` },
  twitter: { card: 'summary_large_image', title, description: appDescription, images: `${siteUrl}/hero.png` },
};

export default function HomePage() {
  const version = latestRelease();

  return (
    <main className="flex flex-1 flex-col overflow-x-clip">
      <Hero version={version} />
      <Section
        eyebrow="Structs are the schema"
        title="Your Go types already describe your API."
        intro="grx reads your structs and methods once at startup and builds the schema clients see. Hover a line to follow the edge from Go to GraphQL."
      >
        <SchemaMapper />
      </Section>
      <Section
        eyebrow="Batteries, wired in"
        title="Everything is an option on one handler."
        intro="Start with a schema. Add realtime, auth and guard rails as the service grows, without adopting a framework."
      >
        <OptionsGraph />
      </Section>
      <RequestPath />
      <Section
        eyebrow="Benchmarks"
        title="Same queries. A fraction of the work."
        intro="parse → validate → execute → JSON on identical fixtures for grx, graph-gophers and graphql-go. grx does it with far fewer allocations, so there is less GC pressure under real traffic."
      >
        <SpeedPlot />
        <div className="mt-6 grid gap-4 md:grid-cols-[1fr_1fr_auto]">
          <MigrateLink from="graphql-go/graphql" href="/guides/migrate/from-graphql-go/" />
          <MigrateLink from="graph-gophers/graphql-go" href="/guides/migrate/from-graph-gophers/" />
          <Link
            href="/benchmarks/"
            className="group inline-flex items-center justify-center gap-1.5 rounded-md px-4 py-4 text-sm font-medium text-fd-muted-foreground hover:text-fd-foreground"
          >
            Methodology
            <ArrowRight className="size-4 transition group-hover:translate-x-0.5" />
          </Link>
        </div>
      </Section>
      <QuickStart version={version} />
      <FinalCta />
      <Footer version={version} />
    </main>
  );
}

function Container({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('mx-auto w-full max-w-[1200px] px-5 sm:px-8', className)}>{children}</div>;
}

function Eyebrow({ children }: { children: ReactNode }) {
  return (
    <p className="flex items-center gap-3 text-sm font-medium text-accent">
      <span className="relative flex size-3.5 items-center justify-center rounded-full border-2 border-grx-500">
        <span className="size-1 rounded-full bg-grx-500" />
      </span>
      <span aria-hidden className="h-px w-8 bg-gradient-to-r from-grx-500 to-grx-500/0" />
      {children}
    </p>
  );
}

function Section({
  eyebrow,
  title,
  intro,
  children,
}: {
  eyebrow: string;
  title: string;
  intro: string;
  children: ReactNode;
}) {
  return (
    <section className="py-20 lg:py-28">
      <Container>
        <div className="max-w-3xl">
          <Eyebrow>{eyebrow}</Eyebrow>
          <h2 className="font-display mt-5 text-[clamp(2rem,4.2vw,3.2rem)] font-semibold leading-[1.05]">{title}</h2>
          <p className="mt-5 max-w-2xl text-lg leading-relaxed text-fd-muted-foreground">{intro}</p>
        </div>
        <div className="mt-12 lg:mt-14">{children}</div>
      </Container>
    </section>
  );
}

function Hero({ version }: { version: string }) {
  return (
    <section className="relative isolate overflow-hidden">
      <div
        aria-hidden
        className="grx-dots absolute inset-0 -z-10 [mask-image:radial-gradient(ellipse_70%_60%_at_70%_40%,black,transparent)]"
      />
      <div
        aria-hidden
        className="absolute -top-40 right-[-10%] -z-10 h-[38rem] w-[52rem] rounded-full bg-[radial-gradient(closest-side,rgba(99,102,241,0.22),transparent)]"
      />
      <Container className="grid items-center gap-14 py-16 lg:grid-cols-[0.9fr_1.1fr] lg:py-24">
        <div className="grx-fade-up min-w-0">
          {version && (
            <a
              href={`${githubUrl}/releases/tag/${version}`}
              className="group inline-flex items-center gap-2 rounded-md border border-fd-border bg-fd-background/70 py-1 pl-2 pr-3 text-xs font-medium text-fd-muted-foreground backdrop-blur transition hover:border-grx-400/60 hover:text-fd-foreground"
            >
              <span className="grx-pulse size-2 rounded-full bg-grx-500" />
              grx {version} is out
              <ArrowRight className="size-3 transition group-hover:translate-x-0.5" />
            </a>
          )}
          <h1 className="font-display mt-6 text-[clamp(2.7rem,5.4vw,4.4rem)] font-semibold leading-[1.02]">
            GraphQL, resolved
            <br />
            in <span className="text-accent">plain Go.</span>
          </h1>
          <p className="mt-6 max-w-[32rem] text-lg leading-relaxed text-fd-muted-foreground">
            Your structs are the schema and your methods are the resolvers. grx serves them as a fast GraphQL API
            with subscriptions, persisted queries and guard rails, from an ordinary{' '}
            <code className="font-mono text-[0.9em] text-fd-foreground">http.Handler</code>.
          </p>
          <div className="mt-8 flex flex-wrap items-center gap-3">
            <Link
              href="/getting-started/"
              className="group inline-flex items-center gap-2 rounded-md bg-grx-600 px-5 py-2.5 text-sm font-semibold text-white shadow-lg shadow-grx-600/25 transition hover:bg-grx-500"
            >
              Get started
              <ArrowRight className="size-4 transition group-hover:translate-x-0.5" />
            </Link>
            <a
              href={githubUrl}
              className="inline-flex items-center gap-1.5 rounded-md border border-fd-border bg-fd-background/70 px-5 py-2.5 text-sm font-semibold backdrop-blur transition hover:border-fd-foreground/25"
            >
              View on GitHub
              <ArrowUpRight className="size-4 text-fd-muted-foreground" />
            </a>
          </div>
          <div className="mt-8">
            <InstallCommand />
          </div>
        </div>
        <div className="grx-fade-up min-w-0 [animation-delay:120ms]">
          <QueryGraph />
        </div>
      </Container>
    </section>
  );
}

const stages = [
  { name: 'Your router', note: 'net/http, Chi, Gin, Echo, Fiber', own: true },
  { name: 'Transport', note: 'HTTP, WebSocket, SSE' },
  { name: 'Parse', note: 'persisted queries' },
  { name: 'Validate', note: 'depth & size limits' },
  { name: 'Execute', note: 'authorizers, plugins' },
  { name: 'Your resolvers', note: 'plain Go methods', own: true },
];

const routers = [
  { name: 'net/http', href: '/getting-started/basic-http/' },
  { name: 'ServeMux', href: '/getting-started/servemux/' },
  { name: 'Chi', href: '/getting-started/chi/' },
  { name: 'Gin', href: '/getting-started/gin/' },
  { name: 'Echo', href: '/getting-started/echo/' },
  { name: 'Fiber', href: '/getting-started/fiber/' },
];

function RequestPath() {
  return (
    <section className="border-y border-fd-border bg-fd-card/40 py-20 lg:py-28">
      <Container>
        <div className="max-w-3xl">
          <Eyebrow>Fits your stack</Eyebrow>
          <h2 className="font-display mt-5 text-[clamp(2rem,4.2vw,3.2rem)] font-semibold leading-[1.05]">
            grx owns GraphQL. You own everything else.
          </h2>
          <p className="mt-5 max-w-2xl text-lg leading-relaxed text-fd-muted-foreground">
            Routing, TLS, CORS, auth extraction and your database stay where they already live. grx sits in the
            middle of the request and nowhere else.
          </p>
        </div>

        <div className="relative mt-14">
          <div aria-hidden className="absolute left-[7px] top-2 bottom-2 w-px bg-fd-border md:left-[8.33%] md:right-[8.33%] md:top-[7px] md:bottom-auto md:h-px md:w-auto">
            <span className="grx-packet-y absolute -left-[3px] size-[7px] rounded-full bg-grx-500 shadow-[0_0_10px_2px_rgba(99,102,241,0.6)] md:hidden" />
            <span className="grx-packet-x absolute -top-[3px] hidden size-[7px] rounded-full bg-grx-500 shadow-[0_0_10px_2px_rgba(99,102,241,0.6)] md:block" />
          </div>
          <ol className="relative grid gap-7 md:grid-cols-6 md:gap-4">
            {stages.map((s) => (
              <li key={s.name} className="flex items-start gap-4 md:flex-col md:items-center md:text-center">
                <span
                  className={cn(
                    'relative z-10 flex size-[15px] shrink-0 items-center justify-center rounded-full border-2',
                    s.own ? 'border-fd-muted-foreground/50 bg-fd-background' : 'border-grx-500 bg-grx-500',
                  )}
                />
                <div className="-mt-1 md:mt-0">
                  <p className="font-semibold">{s.name}</p>
                  <p className="mt-0.5 text-sm text-fd-muted-foreground">{s.note}</p>
                </div>
              </li>
            ))}
          </ol>
        </div>

        <div className="mt-14 flex flex-wrap items-center gap-2">
          <span className="me-2 text-sm text-fd-muted-foreground">Mount it on</span>
          {routers.map((r) => (
            <Link
              key={r.name}
              href={r.href}
              className="rounded-md border border-fd-border bg-fd-background px-3.5 py-1.5 font-mono text-xs transition hover:border-grx-400/60 hover:text-accent"
            >
              {r.name}
            </Link>
          ))}
        </div>
      </Container>
    </section>
  );
}

function MigrateLink({ from, href }: { from: string; href: string }) {
  return (
    <Link
      href={href}
      className="group flex items-center justify-between gap-4 rounded-md border border-fd-border px-5 py-4 transition hover:border-grx-400/60"
    >
      <span>
        <span className="block text-xs text-fd-muted-foreground">Migrating from</span>
        <span className="font-mono text-sm">{from}</span>
      </span>
      <ArrowRight className="size-4 shrink-0 text-fd-muted-foreground transition group-hover:translate-x-0.5 group-hover:text-accent" />
    </Link>
  );
}

function QuickStart({ version }: { version: string }) {
  return (
    <section className="py-20 lg:py-28">
      <Container className="grid items-center gap-12 lg:grid-cols-[0.85fr_1.15fr]">
        <div>
          <Eyebrow>Five minutes</Eyebrow>
          <h2 className="font-display mt-5 text-[clamp(2rem,4.2vw,3.2rem)] font-semibold leading-[1.05]">
            From empty folder to a live API.
          </h2>
          <ul className="mt-8 space-y-5">
            {[
              ['One module', 'go get it. Go 1.25 or newer.'],
              ['GraphiQL built in', 'on any path you choose.'],
              ['Plain JSON over HTTP', 'so any GraphQL client works, or curl.'],
            ].map(([a, b]) => (
              <li key={a} className="flex gap-4">
                <span className="mt-1.5 flex size-3.5 shrink-0 items-center justify-center rounded-full border-2 border-grx-500">
                  <span className="size-1 rounded-full bg-grx-500" />
                </span>
                <span className="text-fd-muted-foreground">
                  <span className="font-medium text-fd-foreground">{a}:</span> {b}
                </span>
              </li>
            ))}
          </ul>
          <Link
            href="/getting-started/"
            className="group mt-9 inline-flex items-center gap-2 rounded-md bg-fd-foreground px-5 py-2.5 text-sm font-semibold text-fd-background transition hover:opacity-90"
          >
            Follow the walkthrough
            <ArrowRight className="size-4 transition group-hover:translate-x-0.5" />
          </Link>
        </div>
        <TerminalDemo version={version || 'v0.8.1'} />
      </Container>
    </section>
  );
}

function FinalCta() {
  return (
    <section className="pb-24">
      <Container>
        <div className="grx-night-grid grid items-center gap-10 overflow-hidden rounded-lg border border-grx-300/15 px-6 py-14 text-white sm:px-12 lg:grid-cols-[minmax(0,1fr)_minmax(0,20rem)] lg:py-16 xl:grid-cols-[minmax(0,1fr)_minmax(0,24rem)]">
          <div className="min-w-0 max-w-xl">
            <p className="flex items-center gap-3 text-sm font-medium text-grx-300">
              <span className="flex size-3.5 items-center justify-center rounded-full border-2 border-grx-400">
                <span className="size-1 rounded-full bg-grx-400" />
              </span>
              <span aria-hidden className="h-px w-8 bg-gradient-to-r from-grx-400 to-grx-400/0" />
              Your turn
            </p>
            <h2 className="font-display mt-5 text-[clamp(2.2rem,4.6vw,3.6rem)] font-semibold leading-[1.04]">
              Resolve your first query today.
            </h2>
            <p className="mt-5 text-lg text-white/65">
              Open source, MIT licensed, and built to live inside the Go service you already run.
            </p>
            <div className="mt-9 flex flex-wrap gap-3">
              <Link
                href="/getting-started/"
                className="group inline-flex items-center gap-2 rounded-md bg-white px-5 py-2.5 text-sm font-semibold text-night transition hover:bg-grx-100"
              >
                Get started
                <ArrowRight className="size-4 transition group-hover:translate-x-0.5" />
              </Link>
              <Link
                href="/examples/"
                className="inline-flex items-center gap-2 rounded-md border border-white/20 px-5 py-2.5 text-sm font-semibold transition hover:bg-white/10"
              >
                Browse examples
              </Link>
            </div>
          </div>
          <GrxMark className="pointer-events-none hidden h-auto w-full lg:block" />
        </div>
      </Container>
    </section>
  );
}

function Footer({ version }: { version: string }) {
  return (
    <footer className="border-t border-fd-border">
      <Container className="flex flex-col gap-4 py-8 text-sm text-fd-muted-foreground sm:flex-row sm:items-center sm:justify-between">
        <p className="flex items-center gap-2">
          <GrxMark className="h-5 w-auto" strokeWidth={6} />
          grx{version && ` ${version}`} · MIT · GraphQL October 2021 spec
        </p>
        <nav className="flex flex-wrap gap-x-5 gap-y-2">
          <Link href="/getting-started/" className="hover:text-fd-foreground">
            Docs
          </Link>
          <Link href="/reference/" className="hover:text-fd-foreground">
            API reference
          </Link>
          <Link href="/benchmarks/" className="hover:text-fd-foreground">
            Benchmarks
          </Link>
          <a href={`${githubUrl}/blob/main/CHANGELOG.md`} className="hover:text-fd-foreground">
            Changelog
          </a>
          <a href={githubUrl} className="hover:text-fd-foreground">
            GitHub
          </a>
        </nav>
      </Container>
    </footer>
  );
}
