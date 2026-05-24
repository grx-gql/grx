import type { DefaultTheme } from "vitepress";
import { defineConfig } from "vitepress";
import { withMermaid } from "vitepress-plugin-mermaid";

/** Sidebar: Start here → Concepts → Getting Started → Guides (grouped) → reference & project. */
const docsSidebar: DefaultTheme.SidebarItem[] = [
  {
    text: "Start here",
    collapsed: false,
    items: [
      {
        text: "What grx is · feature surface",
        link: "/concepts/what-is-grx",
      },
      {
        text: "Run your first server",
        link: "/getting-started/",
      },
    ],
  },
  {
    text: "Understanding GraphQL",
    collapsed: false,
    items: [
      {
        text: "How GraphQL backends work (FAQ)",
        link: "/guides/graphql-backend-essentials",
      },
    ],
  },
  {
    text: "Concepts",
    collapsed: false,
    items: [
      { text: "Routers & transports", link: "/concepts/transports" },
      { text: "Benchmarks", link: "/benchmarks" },
      { text: "Organize your code", link: "/concepts/schema-mapping" },
      { text: "Resolver methods", link: "/concepts/resolvers" },
      { text: "Plugins & hooks", link: "/concepts/plugins" },
      { text: "Architecture", link: "/concepts/architecture" },
    ],
  },
  {
    text: "Getting Started",
    collapsed: true,
    items: [
      { text: "Basic · net/http", link: "/getting-started/basic-http" },
      { text: "ServeMux", link: "/getting-started/servemux" },
      { text: "Chi", link: "/getting-started/chi" },
      { text: "Gin", link: "/getting-started/gin" },
      { text: "Echo", link: "/getting-started/echo" },
      { text: "Fiber", link: "/getting-started/fiber" },
    ],
  },
  {
    text: "Guides",
    collapsed: true,
    items: [
      {
        text: "Query and mutation",
        collapsed: false,
        items: [
          {
            text: "Queries and mutations",
            link: "/guides/query-mutation-server",
          },
          {
            text: "Query & document limits",
            link: "/guides/query-limits",
          },
          {
            text: "Resolver batching & N+1",
            link: "/guides/resolver-batching",
          },
          {
            text: "Testing with the HTTP client",
            link: "/guides/testing",
          },
        ],
      },
      {
        text: "Subscriptions",
        collapsed: false,
        items: [
          {
            text: "Realtime subscriptions",
            link: "/guides/subscriptions",
          },
          {
            text: "Pub/sub backends (memory vs Redis)",
            link: "/concepts/pubsub#choosing-a-backend",
          },
        ],
      },
      {
        text: "Schema",
        collapsed: false,
        items: [
          {
            text: "Define your schema",
            link: "/concepts/schema-basics",
          },
          { text: "Custom scalars", link: "/guides/custom-scalars" },
          {
            text: "Built-in directives",
            link: "/guides/schema-directives",
          },
        ],
      },
      {
        text: "Other guides",
        collapsed: false,
        items: [
          {
            text: "AI assistants",
            link: "/guides/ai-assistants",
          },
          {
            text: "Security",
            link: "/guides/production-security",
          },
          {
            text: "Introspection",
            link: "/guides/introspection",
          },
          {
            text: "Limits",
            link: "/guides/request-limits",
          },
          {
            text: "Deployment",
            link: "/guides/deployment",
          },
          {
            text: "Authentication & authorization",
            link: "/guides/auth",
          },
          { text: "File uploads", link: "/guides/file-upload" },
          {
            text: "Persisted queries (APQ)",
            link: "/guides/persisted-queries",
          },
          {
            text: "Errors & client responses",
            link: "/guides/errors-and-masking",
          },
          { text: "CORS & browsers", link: "/guides/cors-browsers" },
          { text: "Custom plugin", link: "/guides/custom-plugin" },
          { text: "Custom transport", link: "/guides/custom-transport" },
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
    ],
  },
  {
    text: "API reference",
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
      { text: "http", link: "/reference/http/" },
      { text: "sse", link: "/reference/sse/" },
      { text: "websocket", link: "/reference/websocket/" },
      { text: "memory-pubsub", link: "/reference/memory-pubsub/" },
      { text: "redis-pubsub", link: "/reference/redis-pubsub/" },
    ],
  },
  {
    text: "Project",
    collapsed: true,
    items: [
      { text: "Examples", link: "/examples/" },
      { text: "Roadmap", link: "/roadmap" },
      { text: "Changelog", link: "/changelog" },
    ],
  },
  {
    text: "Under the hood",
    collapsed: true,
    items: [
      { text: "Execution pipeline", link: "/concepts/execution" },
      {
        text: "Subscriptions (runtime)",
        link: "/concepts/subscriptions",
      },
      { text: "Pub/Sub", link: "/concepts/pubsub" },
    ],
  },
];

/** Examples sidebar: runnable categories + outbound links only. */
const examplesSidebar: DefaultTheme.SidebarItem[] = [
  {
    text: "Starter servers",
    items: [
      { text: "Basic GraphQL HTTP", link: "/examples/#examples-basic-http" },
      {
        text: "Subscriptions",
        link: "/examples/#examples-subscriptions",
      },
      {
        text: "Bearer auth + field guards",
        link: "/examples/#examples-auth",
      },
    ],
  },
  {
    text: "Router integrations",
    items: [
      {
        text: "net/http ServeMux",
        link: "/examples/#examples-router-servemux",
      },
      { text: "Chi", link: "/examples/#examples-router-chi" },
      { text: "Gin", link: "/examples/#examples-router-gin" },
      { text: "Echo", link: "/examples/#examples-router-echo" },
      { text: "Fiber", link: "/examples/#examples-router-fiber" },
    ],
  },
  {
    text: "Source",
    items: [
      {
        text: "GitHub repository",
        link: "https://github.com/grx-gql/grx",
      },
      {
        text: "Discussions",
        link: "https://github.com/grx-gql/grx/discussions",
      },
    ],
  },
];

// https://vitepress.dev/reference/site-config
export default withMermaid(
  defineConfig({
    title: "grx",
    // Browser tabs: descriptive page titles with a fixed brand suffix (Hono-style separator).
    titleTemplate: ":title · grx",
    description:
      "GraphQL servers in Go: structs as types, built-in subscriptions, zero third-party runtime dependencies.",
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

      // Top bar: Docs / Examples / Discussions (hono.dev-style).
      nav: [
        {
          text: "Docs",
          link: "/getting-started/",
          activeMatch:
            "^/(getting-started|concepts|guides|benchmarks|roadmap|changelog|reference)(/|$)|^/$",
        },
        { text: "Examples", link: "/examples/", activeMatch: "^/examples" },
        {
          text: "Discussions",
          link: "https://github.com/grx-gql/grx/discussions",
          target: "_blank",
          rel: "noopener noreferrer",
        },
      ],

      sidebar: {
        "/examples/": examplesSidebar,
        "/": docsSidebar,
      },

      socialLinks: [
        { icon: "github", link: "https://github.com/grx-gql/grx" },
      ],

      editLink: {
        pattern:
          "https://github.com/grx-gql/grx/edit/main/docs/:path",
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
