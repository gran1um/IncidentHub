const path = require('path');

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'IncidentHub Docs',
  tagline: 'Operational documentation and API reference',
  favicon: 'img/favicon.png',
  url: 'http://localhost',
  baseUrl: '/',
  onBrokenLinks: 'warn',
  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },
  i18n: {
    defaultLocale: 'ru',
    locales: ['ru'],
  },
  customFields: {
    openapiSpecUrl: '/openapi.json',
  },
  presets: [
    [
      'classic',
      {
        docs: {
          id: 'guide',
          path: path.resolve(__dirname, 'docs-generated'),
          routeBasePath: 'guide',
          sidebarPath: require.resolve('./sidebars.js'),
          include: ['**/*.md'],
          exclude: [],
          editUrl: undefined,
        },
        blog: false,
        theme: {
          customCss: require.resolve('./src/css/custom.css'),
        },
      },
    ],
  ],
  themeConfig: {
    navbar: {
      title: 'IncidentHub Docs',
      items: [
        { to: '/guide/intro', label: 'Docs Home', position: 'left' },
        { to: '/guide/overview/project-overview', label: 'Overview', position: 'left' },
        { to: '/guide/architecture/backend', label: 'Backend', position: 'left' },
        { to: '/guide/architecture/frontend', label: 'Frontend', position: 'left' },
        { to: '/guide/architecture/ai-agent-queue', label: 'AI Agents', position: 'left' },
        { to: '/api', label: 'API Reference', position: 'left' },
      ],
    },
  },
};

module.exports = config;
