import { copyFileSync, mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const source = resolve(__dirname, '../../backend/internal/api/openapi.json');
const target = resolve(__dirname, '../static/openapi.json');

mkdirSync(dirname(target), { recursive: true });
copyFileSync(source, target);
console.log(`[docs-web] synced OpenAPI: ${source} -> ${target}`);
