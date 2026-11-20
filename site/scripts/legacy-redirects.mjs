// SPDX-License-Identifier: Apache-2.0

// Writes dist/<old>/index.html refresh pages for the doc URLs this site published
// at the root before they moved under /docs. The CloudFront viewer-request
// function rewrites a directory path to index.html and does not 301 these
// paths, so the object has to exist or the first sync --delete drops them.
// A browser refresh is not an HTTP redirect.

import { existsSync, mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const siteDir = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const dist = join(siteDir, 'dist');

// Paths the root-built site emitted. New pages do not belong here.
const oldPaths = [
  'quick-start',
  'gate',
  'claims-and-leases',
  'config',
  'cli',
  'recipes',
  'recipes/claude-code-agent-teams',
  'recipes/copilot-cloud-agent',
];

const origin = 'https://getwaggle.dev';

function refreshPage(target) {
  const canonical = origin + target;
  return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <title>Moved</title>
  <link rel="canonical" href="${canonical}" />
  <meta http-equiv="refresh" content="0; url=${target}" />
  <script>location.replace(${JSON.stringify(target)})</script>
</head>
<body>
  <p><a href="${target}">This page has moved.</a></p>
</body>
</html>
`;
}

if (!existsSync(dist)) {
  console.error('legacy-redirects: dist/ is missing; run astro build first');
  process.exit(1);
}

for (const oldPath of oldPaths) {
  const target = '/docs/' + oldPath + '/';
  const out = join(dist, oldPath, 'index.html');
  if (existsSync(out)) {
    console.error(`legacy-redirects: ${oldPath}/index.html already exists; not overwriting`);
    process.exit(1);
  }
  mkdirSync(dirname(out), { recursive: true });
  writeFileSync(out, refreshPage(target));
}
console.log(`legacy-redirects: wrote ${oldPaths.length} root refresh pages`);
