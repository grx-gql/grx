import type { Metadata } from 'next';
import { DocsPageView, docsPageMetadata } from '@/components/docs-page';
import { source } from '@/lib/source';
import { isReferenceSlug } from '@/lib/trees';

export default async function Page(props: PageProps<'/reference/[[...slug]]'>) {
  const { slug = [] } = await props.params;
  return <DocsPageView slugs={['reference', ...slug]} />;
}

export async function generateStaticParams() {
  return source
    .generateParams()
    .filter((p) => isReferenceSlug(p.slug))
    .map((p) => ({ slug: p.slug.slice(1) }));
}

export async function generateMetadata(props: PageProps<'/reference/[[...slug]]'>): Promise<Metadata> {
  const { slug = [] } = await props.params;
  return docsPageMetadata(['reference', ...slug]);
}
