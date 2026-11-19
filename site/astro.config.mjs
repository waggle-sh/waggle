// SPDX-License-Identifier: Apache-2.0

import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// The public URL. SITE_URL overrides it (see .env.example), e.g. for preview deploys.
const site = process.env.SITE_URL || 'https://getwaggle.dev';

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
