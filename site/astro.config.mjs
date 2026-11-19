// SPDX-License-Identifier: Apache-2.0

import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// The public URL is not decided yet, so it is never written here. Set SITE_URL in the build
// environment (see .env.example). Unset, the build still works and emits no canonical URLs or sitemap.
const site = process.env.SITE_URL || undefined;

export default defineConfig({
  site,
  integrations: [
    starlight({
      title: 'Waggle',
      description: 'Task claims and a merge gate for AI coding agents on Git.',
      favicon: '/favicon.svg',
      customCss: ['./src/assets/brand/tokens.css', './src/styles/site.css'],
      sidebar: [
        { label: 'Quick start', slug: 'quick-start' },
        {
          label: 'Concepts',
          items: [
            { label: 'The gate', slug: 'gate' },
            { label: 'Claims and leases', slug: 'claims-and-leases' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'Configuration', slug: 'config' },
            { label: 'CLI', slug: 'cli' },
          ],
        },
        {
          label: 'Recipes',
          items: [
            { label: 'Overview', slug: 'recipes' },
            { label: 'Claude Code agent teams', slug: 'recipes/claude-code-agent-teams' },
            { label: 'Copilot cloud agent', slug: 'recipes/copilot-cloud-agent' },
          ],
        },
      ],
    }),
  ],
});
