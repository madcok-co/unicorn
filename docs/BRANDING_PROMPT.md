# Unicorn Framework — Branding Design Prompt

> Copy-paste this into Claude, ChatGPT, or any AI design tool.

---

## Project: Unicorn Framework

**Tagline:** Batteries-included Go framework. Write business logic, not boilerplate.

**GitHub:** github.com/madcok-co/unicorn  
**Language:** Go  
**License:** MIT  
**Target audience:** Backend engineers, Go developers, startups, indie makers

---

## Brand Personality

| Trait | Description |
|-------|-------------|
| **Minimal** | Clean, no fluff, just what you need |
| **Powerful** | Production-ready, not a toy |
| **Fast** | ~38ns/op, zero-allocation context pooling |
| **Friendly** | Approachable, not enterprise-cold |
| **Magical** | "How did this just work?" feeling |

**Tone:** Confident but not arrogant. Technical but not dry. Think Vercel/Supabase energy, not Oracle/IBM.

**One-liner vibe:** "Spring Boot for Go, but actually fast."

---

## What to Design

### 1. Primary Logo
- A unicorn head/mark + "Unicorn" wordmark
- The unicorn should be abstract/geometric, NOT a cartoon or clipart
- Should reference Go's simplicity — maybe a gopher silhouette merged with a unicorn horn? Or a geometric unicorn horn integrated with Go's blue color
- **Must work at 32x32px** (for favicon/GitHub avatar) and scale up

**Ideas to explore:**
- A stylized unicorn horn forming the letter "U"
- A polygonal/geometric unicorn head (low-poly style)
- A unicorn horn + Go gopher blue color
- An abstract "U" with a horn element

### 2. Icon/Mark Only (no text)
- Square version for GitHub org avatar, favicon, app icon
- Must be recognizable at 16x16px

### 3. Color Palette

Primary colors should feel tech-forward but warm:

| Role | Suggested | Hex |
|------|-----------|-----|
| **Primary** | Electric violet / indigo | `#7C3AED` or `#6366F1` |
| **Accent** | Cyan / teal | `#00D4FF` or `#06B6D4` |
| **Background** | Near-black (dark theme) | `#0A0A0F` |
| **Surface** | Dark gray | `#111118` |
| **Text** | White / light gray | `#E4E4E7` |
| **Success** | Emerald | `#10B981` |
| **Warning** | Amber | `#F59E0B` |
| **Error** | Rose | `#F43F5E` |

> The palette should support both dark-first and light themes.

### 4. Typography
- **Display/Logo:** Geometric sans-serif (Space Grotesk, Inter, or similar)
- **Code:** Monospace (JetBrains Mono, Fira Code, or similar)
- **Body:** Clean sans-serif (Inter, SF Pro, system-ui)

### 5. Social Preview (OG Image)
- 1200x630px for GitHub social preview / Twitter cards
- Logo centered + tagline "Write business logic, not boilerplate"
- Dark background, cyan/violet gradient accent
- Subtle Go gopher reference (optional)

### 6. Badge/Shield
- For README: "powered by unicorn" badge in Go's blue
- Compact version for project footers

---

## References / Mood Board

**Frameworks with great branding to reference:**
- **Echo** (Go) — clean, minimal wordmark
- **Fiber** (Go) — simple, fast-looking
- **Laravel** — elegant, recognizable icon
- **Vercel** — geometric triangle, dark theme
- **Supabase** — playful but professional
- **Prisma** — geometric, colorful

**What to AVOID:**
- Cartoon unicorns (childish)
- Rainbow gradients (cliché)
- Overly complex illustrations
- Thin/light fonts (hard to read at small sizes)
- Anything that looks like a kids' toy

---

## Deliverables Requested

1. **Logo SVG** — `logo.svg` (full lockup: icon + wordmark), `logo-icon.svg` (icon only)
2. **Dark/light variants** — `logo-dark.svg`, `logo-light.svg` for different backgrounds
3. **Favicon** — 32x32 and 16x16 PNG or SVG
4. **OG Image** — 1200x630 PNG
5. **Color palette export** — as CSS variables or Tailwind config
6. **Typography recommendations** — Google Fonts links

---

## Usage Context

The logo will appear in:
- **GitHub README** (centered, large)
- **GitHub org avatar** (square, small)
- **Docs site** (VitePress header)
- **Go package documentation** (pkg.go.dev)
- **Social media** (Twitter/X, LinkedIn posts)
- **Terminal/CLI** (ASCII art version would be sick)

---

## Sample Prompt if Using DALL-E / Midjourney

```
Minimal geometric unicorn logo for a Go programming framework.
Dark background. Abstract polygonal unicorn head with a single horn,
integrated with cyan (#00D4FF) accent. Clean lines, no gradients,
flat design. Square composition. Professional, tech-forward.
No text. Vector style.
```
