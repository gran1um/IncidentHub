import React, { useEffect } from 'react';
import Layout from '@theme/Layout';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';

function loadSwaggerUI(specUrl) {
  if (typeof window === 'undefined') {
    return () => {};
  }

  const existingStyle = document.querySelector('link[data-incidenthub-swagger-style="true"]');
  const existingScript = document.querySelector('script[data-incidenthub-swagger-script="true"]');

  const style = existingStyle || document.createElement('link');
  style.setAttribute('data-incidenthub-swagger-style', 'true');
  style.rel = 'stylesheet';
  style.href = 'https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css';
  if (!existingStyle) {
    document.head.appendChild(style);
  }

  const bootstrap = () => {
    if (window.SwaggerUIBundle) {
      window.SwaggerUIBundle({
        url: specUrl,
        dom_id: '#incidenthub-swagger-ui',
        deepLinking: true,
        defaultModelsExpandDepth: 1,
        defaultModelExpandDepth: 1,
        docExpansion: 'none',
        displayRequestDuration: true,
        tryItOutEnabled: false,
      });
    }
  };

  if (existingScript) {
    bootstrap();
    return () => {};
  }

  const script = document.createElement('script');
  script.setAttribute('data-incidenthub-swagger-script', 'true');
  script.src = 'https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js';
  script.async = true;
  script.onload = bootstrap;
  document.body.appendChild(script);

  return () => {
    // Keep assets cached after first render.
  };
}

export default function ApiReferencePage() {
  const { siteConfig } = useDocusaurusContext();
  const specUrl = siteConfig?.customFields?.openapiSpecUrl || '/openapi.json';

  useEffect(() => loadSwaggerUI(specUrl), [specUrl]);

  return (
    <Layout title="API Reference" description="IncidentHub API Reference">
      <main className="container margin-vert--lg">
        <h1>API Reference</h1>
        <p>
          Спецификация синхронизируется из backend OpenAPI и отображается в режиме документации.
        </p>
        <div id="incidenthub-swagger-ui" className="incidenthub-swagger-ui" />
      </main>
    </Layout>
  );
}
