// @ts-check
import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

export default defineConfig({
  site: "https://patrickkabwe.github.io",
  base: "/grx",
  trailingSlash: "ignore",
  integrations: [
    starlight({
      title: "grx",
      description:
        "A fast, dependency-free Go GraphQL server and runtime — built for clarity and predictable performance.",
      logo: {
        src: "./src/assets/logo.svg",
        replacesTitle: true,
      },
      favicon: "/favicon.svg",
      social: [
        {
          icon: "github",
          label: "GitHub",
          href: "https://github.com/patrickkabwe/grx",
        },
        {
          icon: "seti:go",
          label: "pkg.go.dev",
          href: "https://pkg.go.dev/github.com/patrickkabwe/grx",
        },
      ],
      editLink: {
        baseUrl: "https://github.com/patrickkabwe/grx/edit/main/docs/",
      },
      lastUpdated: true,
      pagination: true,
      tableOfContents: { minHeadingLevel: 2, maxHeadingLevel: 4 },
      customCss: ["./src/styles/custom.css"],
      expressiveCode: {
        themes: ["github-dark", "github-light"],
        styleOverrides: {
          borderRadius: "0.5rem",
          codeFontSize: "0.875rem",
        },
      },
      sidebar: [
        { label: "Introduction", link: "/" },
        { label: "Getting Started", link: "/getting-started/" },
        {
          label: "Concepts",
          autogenerate: { directory: "concepts" },
        },
        {
          label: "Guides",
          autogenerate: { directory: "guides" },
        },
        { label: "Benchmarks", link: "/benchmarks/" },
        { label: "Roadmap", link: "/roadmap/" },
        {
          label: "API Reference",
          autogenerate: { directory: "reference" },
          collapsed: true,
        },
        { label: "Changelog", link: "/changelog/", badge: "new" },
      ],
    }),
  ],
});
