import { source } from '@/lib/source';
import {
  DocsBody,
  DocsDescription,
  DocsPage,
  DocsTitle,
  MarkdownCopyButton,
  PageLastUpdate,
  ViewOptionsPopover,
} from 'fumadocs-ui/layouts/docs/page';
import { notFound } from 'next/navigation';
import { getMDXComponents } from '@/components/mdx';
import type { Metadata } from 'next';
import { createRelativeLink } from 'fumadocs-ui/mdx';
import { getPageImageUrl, getPageMarkdownUrl, gitConfig, githubUrl, siteUrl } from '@/lib/shared';

export function DocsPageView({ slugs }: { slugs: string[] | undefined }) {
  const page = source.getPage(slugs);
  if (!page) notFound();

  const MDX = page.data.body;
  const markdownUrl = getPageMarkdownUrl(page).url;
  const filePath = `docs/content/docs/${page.path}`;

  return (
    <DocsPage toc={page.data.toc} full={page.data.full} tableOfContent={{ style: 'clerk' }}>
      <DocsTitle>{page.data.title}</DocsTitle>
      <DocsDescription className="mb-0">{page.data.description}</DocsDescription>
      <div className="flex flex-row gap-2 items-center border-b pb-6">
        <MarkdownCopyButton markdownUrl={markdownUrl} />
        <ViewOptionsPopover markdownUrl={markdownUrl} githubUrl={`${githubUrl}/blob/${gitConfig.branch}/${filePath}`} />
      </div>
      <DocsBody>
        <MDX
          components={getMDXComponents({
            a: createRelativeLink(source, page),
          })}
        />
      </DocsBody>
      <div className="flex flex-row flex-wrap items-center justify-between gap-4 text-sm">
        <a
          href={`${githubUrl}/edit/${gitConfig.branch}/${filePath}`}
          target="_blank"
          rel="noreferrer noopener"
          className="text-fd-muted-foreground hover:text-fd-foreground transition-colors"
        >
          Edit this page on GitHub
        </a>
        {page.data.lastModified && <PageLastUpdate date={page.data.lastModified} />}
      </div>
    </DocsPage>
  );
}

export function docsPageMetadata(slugs: string[] | undefined): Metadata {
  const page = source.getPage(slugs);
  if (!page) notFound();

  const image = getPageImageUrl(page).url;

  return {
    title: page.data.title,
    description: page.data.description,
    alternates: { canonical: `${siteUrl}${page.url}` },
    openGraph: {
      title: page.data.title,
      description: page.data.description,
      url: `${siteUrl}${page.url}`,
      images: image,
    },
    twitter: {
      card: 'summary_large_image',
      title: page.data.title,
      description: page.data.description,
      images: image,
    },
  };
}
