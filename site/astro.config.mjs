// SPDX-License-Identifier: Apache-2.0

import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// Public URL comes from SITE_URL (see .env.example). The deploy workflow sets
// https://getwaggle.dev. Unset, the build still works and emits no canonical URLs or sitemap.
const site = process.env.SITE_URL || undefined;

// Starlight serves src/content/docs/ from the site root and has no docs-only base.
// Astro's base would also move the landing page (src/pages/index.astro) and the
// root 404. sync-docs.mjs writes the pages under src/content/docs/docs/, so the
// catch-all route serves them at /docs/ and the sidebar slugs include that prefix.
export default defineConfig({
  site,
  integrations: [
    starlight({
      title: 'Waggle',
      description: 'Task claims and a merge gate for AI coding agents on Git.',
      favicon: '/favicon.svg',
      logo: {
        light: './public/brand/logo/w-mark-light.svg',
        dark: './public/brand/logo/w-mark-dark.svg',
        alt: 'Waggle',
      },
      disable404Route: true,
      customCss: ['./src/assets/brand/tokens.css', './src/styles/site.css'],
      sidebar: [
        { label: 'Overview', slug: 'docs' },
        { label: 'Quick start', slug: 'docs/quick-start' },
        {
          label: 'Concepts',
          items: [
            { label: 'The gate', slug: 'docs/gate' },
            { label: 'Claims and leases', slug: 'docs/claims-and-leases' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'Configuration', slug: 'docs/config' },
            { label: 'CLI', slug: 'docs/cli' },
          ],
        },
        {
          label: 'Recipes',
          items: [
            { label: 'Overview', slug: 'docs/recipes' },
            { label: 'Claude Code agent teams', slug: 'docs/recipes/claude-code-agent-teams' },
            { label: 'Copilot cloud agent', slug: 'docs/recipes/copilot-cloud-agent' },
          ],
        },
      ],
    }),
  ],
});
