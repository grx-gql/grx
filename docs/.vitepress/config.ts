import { defineConfig } from "vitepress";
import { withMermaid } from "vitepress-plugin-mermaid";

// https://vitepress.dev/reference/site-config
export default withMermaid(
  defineConfig({
    title: "grx",
    description:
      "A fast, dependency-free Go GraphQL server and runtime — built for clarity and predictable performance.",
    lang: "en-US",
    base: "/grx/",
    srcDir: ".",
    lastUpdated: true,
    cleanUrls: true,

    ignoreDeadLinks: [/^https?:\/\/localhost(?::\d+)?/],

    markdown: {
      theme: { light: "github-light", dark: "github-dark" },
    },

    server: {
      port: 4321,
    },

    themeConfig: {
      logo: { src: "/hero.svg", alt: "grx" },

      nav: [
        { text: "Getting Started", link: "/getting-started" },
        { text: "Reference", link: "/reference/" },
      ],

      sidebar: [
        { text: "Introduction", link: "/" },
        { text: "Getting Started", link: "/getting-started" },
        {
          text: "Concepts",
          collapsed: false,
          items: [
            { text: "Architecture", link: "/concepts/architecture" },
            { text: "Schema Mapping", link: "/concepts/schema-mapping" },
            { text: "Resolvers", link: "/concepts/resolvers" },
            { text: "Execution", link: "/concepts/execution" },
            { text: "Transports", link: "/concepts/transports" },
            { text: "Subscriptions", link: "/concepts/subscriptions" },
            { text: "Plugins", link: "/concepts/plugins" },
            { text: "Pub/Sub", link: "/concepts/pubsub" },
          ],
        },
        {
          text: "Guides",
          collapsed: false,
          items: [
            {
              text: "Build a Query and Mutation Server",
              link: "/guides/query-mutation-server",
            },
            { text: "Add Subscriptions", link: "/guides/subscriptions" },
            { text: "Write a Custom Plugin", link: "/guides/custom-plugin" },
            { text: "Write a Custom Transport", link: "/guides/custom-transport" },
            {
              text: "Migrate to grx",
              collapsed: true,
              items: [
                { text: "Overview", link: "/guides/migrate/" },
                {
                  text: "From graphql-go",
                  link: "/guides/migrate/from-graphql-go",
                },
                {
                  text: "From graph-gophers",
                  link: "/guides/migrate/from-graph-gophers",
                },
              ],
            },
          ],
        },
        { text: "Benchmarks", link: "/benchmarks" },
        { text: "Roadmap", link: "/roadmap" },
        {
          text: "API Reference",
          collapsed: true,
          items: [
            { text: "Overview", link: "/reference/" },
            { text: "grx", link: "/reference/grx/" },
            { text: "core", link: "/reference/core/" },
            { text: "schema", link: "/reference/schema/" },
            { text: "exec", link: "/reference/exec/" },
            { text: "server", link: "/reference/server/" },
            { text: "plugin", link: "/reference/plugin/" },
            { text: "plugin/logger", link: "/reference/plugin/logger/" },
            { text: "pkg/http", link: "/reference/pkg/http/" },
            { text: "pkg/sse", link: "/reference/pkg/sse/" },
            { text: "pkg/websocket", link: "/reference/pkg/websocket/" },
            { text: "pkg/pubsub", link: "/reference/pkg/pubsub/" },
            { text: "pkg/pubsub/redis", link: "/reference/pkg/pubsub/redis/" },
          ],
        },
        { text: "Changelog", link: "/changelog" },
      ],

      socialLinks: [
        { icon: "github", link: "https://github.com/patrickkabwe/grx" },
      ],

      editLink: {
        pattern:
          "https://github.com/patrickkabwe/grx/edit/main/docs/:path",
        text: "Edit this page on GitHub",
      },

      search: {
        provider: "local",
      },
    },

    mermaid: {
      theme: "neutral",
      themeVariables: {
        primaryColor: "#6366f1",
      },
    },
    mermaidPlugin: {
      class: "mermaid",
    },
  }),
);
