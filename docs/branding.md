# Branding

`SchoolBranding` controls school display name, system title, light/dark logos, favicon, login background, primary/secondary/accent colors, welcome text and certificate identity. FluentHub remains the product/repository name, not a forced tenant-facing brand.

The frontend fetches a public safe projection, applies `--brand-primary`, `--brand-secondary` and `--brand-accent`, and updates the page title. Login, headers, sidebars, all four areas and certificate previews use these variables. Built-in fallbacks preserve contrast when setup is incomplete.

Uploads are authorized admin operations. Servers must sniff MIME, cap dimensions/size, generate storage keys and never trust client filenames as paths. SVG is excluded initially to reduce active-content risk. Color inputs must pass syntax and contrast validation.
