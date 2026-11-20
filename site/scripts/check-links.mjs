// SPDX-License-Identifier: Apache-2.0

// Checks internal links in the built site (dist/). Every root-relative href/src must resolve to a
// file in dist/, and every #fragment must match an id in the target page. Run after `npm run build`.
// Exits 1 on any broken link. External links are not fetched.

import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const dist = join(resolve(dirname(fileURLToPath(import.meta.url)), '..'), 'dist');
if (!existsSync(dist)) {
  console.error('check-links: dist/ not found, run npm run build first');
  process.exit(1);
}

const pages = [];
(function walk(dir) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) walk(p);
    else if (name.endsWith('.html')) pages.push(p);
  }
})(dist);

const idCache = new Map();
function idsOf(file) {
  if (!idCache.has(file)) {
    const html = readFileSync(file, 'utf8');
    idCache.set(file, new Set([...html.matchAll(/\bid=["']([^"']+)["']/g)].map((m) => m[1])));
  }
  return idCache.get(file);
}

function resolveTarget(path) {
  const clean = decodeURI(path);
  const candidates = [join(dist, clean), join(dist, clean, 'index.html'), join(dist, clean + '.html')];
  return candidates.find((c) => existsSync(c) && statSync(c).isFile());
}

const errors = [];
for (const page of pages) {
  const html = readFileSync(page, 'utf8');
  const rel = page.slice(dist.length);
  for (const m of html.matchAll(/\b(?:href|src)=["']([^"']+)["']/g)) {
    const href = m[1];
    if (/^(?:[a-z][a-z0-9+.-]*:|\/\/)/i.test(href)) continue; // external, mailto:, data:
    let path, hash;
    if (href.startsWith('#')) {
      path = null;
      hash = href.slice(1);
    } else {
      const u = href.split('#');
      hash = u[1];
      path = u[0].split('?')[0];
      if (!path.startsWith('/')) path = '/' + join(dirname(rel), path).replace(/^\/+/, '');
    }
    const target = path === null ? page : resolveTarget(path);
    if (!target) {
      errors.push(`${rel}: ${href} does not resolve to a file in dist/`);
    } else if (hash && target.endsWith('.html') && !idsOf(target).has(hash)) {
      errors.push(`${rel}: ${href} has no matching #${hash} in the target page`);
    }
  }
}

if (errors.length > 0) {
  for (const e of [...new Set(errors)]) console.error(`check-links: ${e}`);
  process.exit(1);
}
console.log(`check-links: ${pages.length} pages, no broken internal links`);
