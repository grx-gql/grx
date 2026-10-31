import { createGetUrl } from 'fumadocs-core/source';

export const appName = 'grx';
export const appDescription =
  'GraphQL servers in Go where structs are the schema: subscriptions, persisted queries, and production controls on a plain net/http handler.';
// Keep in sync with `basePath` in next.config.mjs.
export const basePath = '/grx';
export const siteUrl = `https://grx-gql.github.io${basePath}`;
export const docsRoute = '/';
export const docsImageRoute = '/og';
export const docsContentRoute = '/llms.mdx';

export const gitConfig = {
  user: 'grx-gql',
  repo: 'grx',
  branch: 'main',
};

export const githubUrl = `https://github.com/${gitConfig.user}/${gitConfig.repo}`;

const getContentUrl = createGetUrl(docsContentRoute);

export function getPageMarkdownUrl(page: { slugs: string[]; locale?: string }) {
  const segments = [...page.slugs, 'content.md'];

  return { segments, url: getContentUrl(segments, page.locale) };
}

const getImageUrl = createGetUrl(docsImageRoute);

export function getPageImageUrl(page: { slugs: string[]; locale?: string }) {
  const segments = [...page.slugs, 'image.png'];

  return { segments, url: getImageUrl(segments, page.locale) };
}
