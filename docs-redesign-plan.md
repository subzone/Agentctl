# Docs Redesign Plan

## Problemi sa trenutnim docs:
1. Standardni Just the Docs - nema ništa specijalno
2. Nema dark/light mode toggle
3. Nema animations za CLI tool koji je vizualno cool
4. Navigacija je basic
5. Nema live examples ili interaktivnih elementata

## Novo rešenje:
1. Napraviti `docs-new/` folder sa modernim static site
2. Koristiti TailwindCSS za styling
3. Dodati dark/light mode toggle sa localStorage
4. Napraviti animirani CLI simulator koji prikazuje `m` TUI
5. Moderna navigacija sa sidebar i breadcrumbs
6. Live code examples sa copy-to-clipboard
7. Build lokalno sa npm/pnpm/yarn
8. Integracija sa GitHub Pages (može koristiti isti `docs/` folder ili novi)

## Struktura:
```
docs-new/
├── package.json          # npm config za TailwindCSS, etc.
├── tailwind.config.js    # Tailwind config
├── postcss.config.js     # PostCSS config
├── src/
│   ├── index.html        # Main template
│   ├── css/
│   │   └── main.css      # Tailwind imports + custom CSS
│   ├── js/
│   │   └── main.js       # Dark mode toggle, animations, etc.
│   ├── pages/
│   │   ├── index.md      -> index.html
│   │   ├── install.md    -> install.html
│   │   ├── quickstart.md -> quickstart.html
│   │   ├── ...           # ostali docs
│   └── assets/
│       ├── images/
│       ├── fonts/
├── build/
│   ├── index.html        # Generated HTML
│   ├── install.html      # Generated HTML
│   ├── ...
├── public/               # Static assets za GitHub Pages
```

## Build process:
- Markdown files se konvertuju u HTML sa template
- TailwindCSS se builduje u single CSS file
- JavaScript se bundle
- Output se stavlja u `build/` ili `public/`

## GitHub Pages:
- Koristi `docs/` folder ili novi `docs-new/build/`
- Dodati workflow za build docs

## Lokalni preview:
- `make setup-docs` - kopira template
- `make docs` - builduje docs
- `make serve-docs` - servira lokalno na http://localhost:3000

## Timeline:
1. Napraviti basic template sa TailwindCSS
2. Konvertovat markdown docs u HTML
3. Dodati dark/light mode
4. Dodati CLI simulator animation
5. Dodati modern navigacija
6. Dodati code examples sa copy button
7. Napraviti build script
8. Test lokalno
9. Integracija sa GitHub Pages