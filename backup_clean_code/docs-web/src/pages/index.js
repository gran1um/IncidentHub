import React from 'react';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';

export default function Home() {
  return (
    <Layout title="IncidentHub Documentation" description="IncidentHub web documentation portal">
      <main className="container margin-vert--xl">
        <h1>IncidentHub Documentation</h1>
        <p>Единый web-портал документации (product, architecture, data, operations, API).</p>
        <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap' }}>
          <Link className="button button--primary" to="/guide/intro">
            Открыть Docs Home
          </Link>
          <Link className="button button--secondary" to="/guide/product/functional-spec">
            Product
          </Link>
          <Link className="button button--secondary" to="/guide/architecture/backend">
            Backend
          </Link>
          <Link className="button button--secondary" to="/guide/architecture/frontend">
            Frontend
          </Link>
          <Link className="button button--secondary" to="/guide/operations/testing">
            Operations
          </Link>
          <Link className="button button--secondary" to="/api">
            Открыть API Reference
          </Link>
        </div>
      </main>
    </Layout>
  );
}
