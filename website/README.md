# DoubleBook Website

Landing page and documentation for DoubleBook, built with [Astro](https://astro.build/).

## Development

```bash
# Install dependencies
npm install

# Start dev server (http://localhost:4321)
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview
```

## Structure

```
website/
├── src/
│   ├── components/     # Astro components
│   ├── layouts/        # Page layouts
│   ├── pages/          # Routes (Astro + MDX)
│   │   ├── docs/       # Documentation pages
│   │   └── developers/ # Developer pages
│   └── styles/         # Global CSS
├── public/             # Static assets
└── astro.config.mjs    # Astro configuration
```

## Adding Content

All content pages use MDX format in the `src/pages/` directory.

### Add a Documentation Page

Create `src/pages/docs/my-page.mdx`:

```mdx
---
layout: '@/layouts/DocsLayout.astro'
title: My Page Title
description: Page description for SEO
---

# My Page Title

Content goes here with full markdown support.

```javascript
// Code blocks have syntax highlighting
const example = true;
```
```

### Update Navigation

Edit `src/components/DocsSidebar.astro` to add links to new pages.

## Deployment

The site is automatically deployed to GitHub Pages when changes are pushed to the `main` branch.

### Manual Deploy

```bash
npm run build
# Output is in dist/
```

### Configuration

Update `astro.config.mjs` with your GitHub username:

```javascript
export default defineConfig({
  site: 'https://yourusername.github.io',
  base: '/gledger',
  // ...
});
```

## Theme

The site uses a retro-pixelated theme with muted blue colors. Key design tokens are in `src/styles/global.css`:

- Fonts: VT323 (display), IBM Plex Mono (body)
- Colors: Muted blue palette (#0d1b2a to #7ba3c9)
- Components: Pixel-styled buttons, cards, and code blocks
