import path from 'node:path';
import { createMDX } from 'fumadocs-mdx/next';

const withMDX = createMDX();
const repoRoot = path.resolve(import.meta.dirname, '..');

/** @type {import('next').NextConfig} */
const config = {
  output: 'export',
  // Served from GitHub Pages at https://grx-gql.github.io/grx/
  basePath: '/grx',
  // Emits `page/index.html` so both `/page` and `/page/` resolve on GitHub Pages.
  trailingSlash: true,
  images: { unoptimized: true },
  reactStrictMode: true,
  // Docs embed runnable sources from ../examples via <include>.
  turbopack: { root: repoRoot },
  outputFileTracingRoot: repoRoot,
};

export default withMDX(config);
