<!-- SPDX-License-Identifier: Apache-2.0 -->
# Brand asset sources

Studio commit: `47a73e740ed256c358048d710fe2955b78631a3d`, directory `products/waggle/`. Each studio file below is a byte for byte copy.

| Copy | Studio path | sha256 |
| :--- | :--- | :--- |
| `site/src/assets/brand/tokens.css` | `tokens.css` | `d6299476471d8ad9b8aa920d62c393b5fb1d7d6cc81c22f1614bccf60a35d43e` |
| `site/src/assets/brand/diagrams/gate-hero.svg` | `diagrams/gate-hero.svg` | `448eb0177f25b94f6aa712ca1ae0310f0f348678a3c976ca74fdafa040a2ea2f` |
| `site/src/assets/brand/diagrams/gate-hero-narrow.svg` | `diagrams/gate-hero-narrow.svg` | `a86b6b05e79ffde3f86d4cf8cc21d2e90ffe71a0b8232f51f864e6f8e3f18c17` |
| `site/src/assets/brand/diagrams/hive-flow.svg` | `diagrams/hive-flow.svg` | `2e04b397752c1c374dbd6ee83e996a0fb55da4877d39134371853f276101665c` |
| `site/src/assets/brand/diagrams/hive-flow-narrow.svg` | `diagrams/hive-flow-narrow.svg` | `ecad8857939b3126b711275bdced235ae3b38e9950bd1771307ac239f4b0ca45` |
| `site/src/assets/brand/mark/waggle-lockup-outlined-mono.svg` | `mark/final/waggle-lockup-outlined-mono.svg` | `753980357e511938d743454c55352414c7df93602af87e14db2da4f699334941` |
| `site/public/favicon.svg` | `mark/final/waggle-favicon.svg` | `ac2b24d8eff0d35874a912ba7409427e51d7ed112a817a76d423f79e8f40f309` |
| `site/public/apple-touch-icon.png` | `mark/final/png/waggle-app-icon-180.png` | `3ffbd918f6098b3b059db56ec21e69010628431604a16a38b96de3ac210bf926` |

The landing page no longer imports `gate-hero.svg` or `gate-hero-narrow.svg`. It uses `site/src/assets/brand/diagrams/checkout-example.svg` and `checkout-example-narrow.svg`, site-authored adaptations of them with the Apple Pay example (they reuse the studio's styles and layout, so they are not byte copies). That directory is the exempt path for diagrams.

## Fonts

The site uses Inter and JetBrains Mono, self-hosted from npm packages pinned in `site/package-lock.json`, not copied into the repo. This replaces the earlier Public Sans and Martian Mono files and their `OFL` texts, which were removed.

| Family | Package | Version | File used | sha256 |
| :--- | :--- | :--- | :--- | :--- |
| Inter, variable weight, latin | `@fontsource-variable/inter` | 5.3.0 | `files/inter-latin-wght-normal.woff2` | `3100e775e8616cd2611beecfa23a4263d7037586789b43f035236a2e6fbd4c62` |
| JetBrains Mono, variable weight, latin | `@fontsource-variable/jetbrains-mono` | 5.3.0 | `files/jetbrains-mono-latin-wght-normal.woff2` | `18be452724bfdc236c074ca94a249a7f41a86752c7d04ab258ce9ed5651f6a7e` |

Each package carries the family's `OFL-1.1` text in its `LICENSE` file (sha256 `3b0a5fca3d17942cde889069889dedbbbd075e9b599968c82a95f4d944e9b345` for Inter, `403581b69dac5cff4079205e01c6b467e56af449ecbd7247693ddb1baafa005b` for JetBrains Mono). The build copies the two woff2 files into `dist/_astro/`. Copies of the two `LICENSE` texts are committed as `site/public/fonts/LICENSE-inter.txt` and `site/public/fonts/LICENSE-jetbrains-mono.txt` (same sha256 as above), so they ship in `dist/fonts/` with the site. Update them when the packages are bumped.

Reserved Font Name: neither copyright line names a reserved name. The `@font-face` family names `Inter` and `JetBrains Mono` are the upstream names. `Public Sans` and `Martian Mono` are also declared in `site/src/styles/site.css`, as local aliases for the same two files, because the byte-copied diagrams name those two families in their own styles.

The Walt kit named in the site brief was not available in this repository; this site keeps the studio's Lease tokens (`tokens.css`) and swaps only the two font families.

## License

Diagrams, marks and tokens are under Apache-2.0 like the product files, covered by the repository `LICENSE`. Apache-2.0 section 6 grants no trademark right, so the mark stays protected as a mark. The fonts are under OFL-1.1, with each license text in its npm package and copied to `site/public/fonts/`.

The Starlight header logo uses `site/src/assets/brand/mark/w-mark-light.svg` and `w-mark-dark.svg`, byte copies of the files of the same name in `site/public/brand/logo/`. Starlight reads logo files from `src/`, and `site/src/assets/brand/mark/` is the exempt path for copied marks.
