/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  guideSidebar: [
    'intro',
    {
      type: 'category',
      label: 'Overview',
      items: ['overview/project-overview', 'overview/documentation-index'],
    },
    {
      type: 'category',
      label: 'Product',
      items: [
        'product/functional-spec',
        'product/platform-functional-inventory',
        'product/designer-screen-spec',
      ],
    },
    {
      type: 'category',
      label: 'Architecture',
      items: [
        'architecture/frontend',
        'architecture/backend',
        'architecture/connectors',
        'architecture/ai',
        'architecture/ai-agent-queue',
        'architecture/frontend-redesign-plan',
      ],
    },
    {
      type: 'category',
      label: 'Data Layer',
      items: ['data/postgresql', 'data/elasticsearch', 'data/redis'],
    },
    {
      type: 'category',
      label: 'Operations',
      items: [
        'operations/observability',
        'operations/infrastructure',
        'operations/security',
        'operations/testing',
        'operations/env-reference',
      ],
    },
    {
      type: 'category',
      label: 'References',
      items: ['references/api-contracts', 'references/i18n-audit'],
    },
    {
      type: 'category',
      label: 'Appendix',
      items: [
        'appendix/backend-loadtest-tooling',
        'appendix/frontend-workflow-studio',
        {
          type: 'category',
          label: 'Loadtest Reports',
          items: [
            'appendix/loadtest-reports/baseline-20260218',
            'appendix/loadtest-reports/normal-20260218',
            'appendix/loadtest-reports/report-async-fix',
            'appendix/loadtest-reports/smoke-quick',
            'appendix/loadtest-reports/stress-20260218',
          ],
        },
      ],
    },
  ],
};

module.exports = sidebars;
