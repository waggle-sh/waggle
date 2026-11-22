// SPDX-License-Identifier: Apache-2.0

// Builds site/src/content/docs/docs/ (generated, gitignored) from two sources:
//   1. the repo's own markdown: docs/gate.md and docs/recipes/*.md, so the site never forks them;
//   2. the pages written for the site in site/content/*.md.
// Each source starts with an SPDX comment and an H1. The H1 becomes the Starlight title, and the
// comment is dropped. Links to other published pages are rewritten to /docs/ routes. The script
// exits 1 on a missing source or a relative .md link it cannot map, so a renamed file fails the build.

import { mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const siteDir = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const repoDir = resolve(siteDir, '..');
const outDir = join(siteDir, 'src', 'content', 'docs');

// Repo-relative source path -> output route (no leading or trailing slash).
const routes = new Map([
  ['docs/gate.md', 'gate'],
  ['docs/recipes/README.md', 'recipes/index'],
  ['docs/recipes/claude-code-agent-teams.md', 'recipes/claude-code-agent-teams'],
  ['docs/recipes/copilot-cloud-agent.md', 'recipes/copilot-cloud-agent'],
  ['docs/recipes/gate-run-receipts.md', 'recipes/gate-run-receipts'],
  ['README.md', 'quick-start'], // the README's Install section lives on the quick start page
]);
const authored = readdirSync(join(siteDir, 'content')).filter((f) => f.endsWith('.md'));
for (const f of authored) routes.set(`site/content/${f}`, f.replace(/\.md$/, ''));

const errors = [];

// Internal notes are never published. docs/rfc-notes and the hive records are not routes, and a
// route that points into them fails the build instead of shipping.
const private_prefixes = ['docs/rfc-notes/', '.waggle/'];
for (const src of routes.keys()) {
  if (private_prefixes.some((p) => src.startsWith(p))) {
    errors.push(`${src}: internal notes must not be published`);
  }
}

function routeUrl(route) {
  const tail = route.replace(/\/?index$/, '');
  return tail === '' ? '/docs/' : '/docs/' + tail + '/';
}

// Absolute site paths that name a published route, with or without the /docs prefix.
// The site root is the landing page and is not a docs route.
function routeFromPath(pathname) {
  let p = pathname;
  if (p.length > 1 && p.endsWith('/')) p = p.slice(0, -1);
  if (p === '/docs') p = '/';
  else if (p.startsWith('/docs/')) p = p.slice('/docs'.length);
  if (!p.startsWith('/')) return null;
  const bare = p.slice(1);
  if (bare === '') return null;
  for (const route of routes.values()) {
    const asDir = route.replace(/\/index$/, '');
    if (bare === route || bare === asDir) return route;
  }
  return null;
}

function rewriteLinks(text, srcPath) {
  let inFence = false;
  return text
    .split('\n')
    .map((line) => {
      if (/^\s*(```|~~~)/.test(line)) inFence = !inFence;
      if (inFence) return line;
      return line.replace(/\]\(([^)\s]+)\)/g, (m, href) => {
        if (/^(https?:|mailto:|#)/.test(href)) return m;
        const hashAt = href.indexOf('#');
        const file = hashAt === -1 ? href : href.slice(0, hashAt);
        const hash = hashAt === -1 ? '' : href.slice(hashAt);
        if (file.startsWith('/')) {
          const route = routeFromPath(file);
          if (!route) return m;
          return `](${routeUrl(route)}${hash})`;
        }
        if (!file.endsWith('.md')) return m;
        const target = resolve(repoDir, dirname(srcPath), file);
        const rel = target.slice(repoDir.length + 1);
        const route = routes.get(rel);
        if (!route) {
          errors.push(`${srcPath}: link ${href} points at ${rel}, which the site does not publish`);
          return m;
        }
        return `](${routeUrl(route)}${hash})`;
      });
    })
    .join('\n');
}

rmSync(outDir, { recursive: true, force: true });

for (const [src, route] of routes) {
  if (src === 'README.md') continue; // only a link target; the page itself is site/content/quick-start.md
  let text;
  try {
    text = readFileSync(join(repoDir, src), 'utf8');
  } catch {
    errors.push(`${src}: source file is missing`);
    continue;
  }
  const lines = text.split('\n');
  if (!lines[0].startsWith('<!-- SPDX-License-Identifier:')) {
    errors.push(`${src}: line 1 must be the SPDX comment`);
    continue;
  }
  const h1 = lines.findIndex((l) => l.startsWith('# '));
  if (h1 < 0) {
    errors.push(`${src}: no H1 to use as the page title`);
    continue;
  }
  const title = lines[h1].slice(2).trim().replace(/"/g, '\\"');
  const body = rewriteLinks(lines.slice(h1 + 1).join('\n').replace(/^\n+/, ''), src);
  const out = join(outDir, 'docs', route + '.md');
  mkdirSync(dirname(out), { recursive: true });
  writeFileSync(out, `---\ntitle: "${title}"\n---\n\n${body}`);
}

// Starlight's built-in /404 route looks up a docs entry with id 404; without one it logs a warning.
// draft: true keeps the entry out of the catch-all docs route, which would otherwise conflict with /404.
writeFileSync(
  join(outDir, '404.md'),
  '---\ntitle: Page not found\ndraft: true\n---\n\nThis page does not exist. Go back to the [home page](/).\n',
);

if (errors.length > 0) {
  for (const e of errors) console.error(`sync-docs: ${e}`);
  process.exit(1);
}
console.log(`sync-docs: wrote ${routes.size - 1} pages under src/content/docs/docs, plus the 404 entry`);
