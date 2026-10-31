import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { DocsPageView, docsPageMetadata } from '@/components/docs-page';
import { source } from '@/lib/source';
import { isReferenceSlug } from '@/lib/trees';

export default async function Page(props: PageProps<'/[...slug]'>) {
  const { slug } = await props.params;
  if (isReferenceSlug(slug)) notFound();
  return <DocsPageView slugs={slug} />;
}

export async function generateStaticParams() {
  return source.generateParams().filter((p) => !isReferenceSlug(p.slug));
}

export async function generateMetadata(props: PageProps<'/[...slug]'>): Promise<Metadata> {
  const { slug } = await props.params;
  return docsPageMetadata(slug);
}
