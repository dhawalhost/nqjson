# nqjson Icon & Branding Assets

**Generated**: October 17, 2025  
**Library**: nqjson - Next-gen Query JSON for Go

---

## 🎨 Available Icons

### 1. **icon-main.svg** (Recommended)
- **Size**: 512x512
- **Style**: Clean, modern with NQ letters
- **Features**: JSON brackets, query arrow, performance indicators
- **Best for**: GitHub profile, documentation, general use
- **Colors**: Cyan (#00B4D8), Blue (#0077B6), Light Cyan (#90E0EF)

### 2. **icon-gradient.svg** (Premium Look)
- **Size**: 512x512
- **Style**: Gradient background with modern typography
- **Features**: Smooth gradients, speed lines, query path
- **Best for**: Website hero, marketing materials, presentations
- **Colors**: Gradient from cyan to deep blue

### 3. **icon-dark.svg** (Dark Theme)
- **Size**: 512x512
- **Style**: Dark background with glowing effect
- **Features**: Glow effect, zero allocation badge
- **Best for**: Dark mode websites, GitHub README (dark theme)
- **Colors**: Dark background with cyan accents

### 4. **icon-minimal.svg** (Simple)
- **Size**: 200x200
- **Style**: Minimal square with bold NQ
- **Features**: Simple, scalable, high contrast
- **Best for**: Favicons, small thumbnails, app icons
- **Colors**: Solid cyan background, white text

### 5. **social-preview.svg** (Social Media)
- **Size**: 1200x630
- **Style**: Horizontal layout for social sharing
- **Features**: Full branding, tagline, feature badges
- **Best for**: GitHub social preview, Twitter cards, Open Graph
- **Colors**: Gradient background with feature highlights

---

## 🎨 Brand Colors

### Primary Palette
```css
--nqjson-cyan:        #00B4D8  /* Main brand color */
--nqjson-deep-blue:   #0077B6  /* Secondary */
--nqjson-light-cyan:  #90E0EF  /* Accents */
--nqjson-sky:         #CAF0F8  /* Light accents */
--nqjson-ice:         #E0F7FA  /* Very light */
```

### Usage Guidelines
- **Primary**: Use cyan (#00B4D8) for main elements, buttons, links
- **Secondary**: Use deep blue (#0077B6) for shadows, borders, depth
- **Accents**: Use light cyan (#90E0EF) for highlights, arrows, decorations
- **Text**: Use white on colored backgrounds, deep blue on light backgrounds

---

## 📏 Icon Specifications

### Main Icon (icon-main.svg)
```
Dimensions: 512x512 pixels
Background: Circle with cyan fill
Main Element: "NQ" letters in white, bold, 160px
Subtitle: "{ }" in light cyan, 80px
Decorations: Query arrow, circuit lines
Format: SVG (vector, infinitely scalable)
```

### Minimal Icon (icon-minimal.svg)
```
Dimensions: 200x200 pixels
Background: Solid cyan square
Main Element: "NQ" letters in white, 100px
Format: SVG, optimized for small sizes
```

### Social Preview (social-preview.svg)
```
Dimensions: 1200x630 pixels (GitHub/OG standard)
Layout: Horizontal
Elements: Logo, tagline, 3 feature badges
Background: Gradient with grid pattern
```

---

## 🖼️ Usage Examples

### In README.md
```markdown
<p align="center">
  <img src="assets/icon-main.svg" width="200" alt="nqjson logo"/>
</p>

# nqjson

Next-gen Query JSON for Go
```

### As Favicon (HTML)
```html
<link rel="icon" type="image/svg+xml" href="/assets/icon-minimal.svg">
```

### GitHub Social Preview
1. Go to: Repository → Settings → Options → Social preview
2. Upload: `assets/social-preview.svg` (or convert to PNG 1200x630)

### In Documentation
```markdown
![nqjson](assets/icon-gradient.svg)
```

---

## 🔧 Converting to Other Formats

### SVG to PNG (High Quality)
```bash
# Using ImageMagick
convert -background none -density 300 assets/icon-main.svg assets/icon-main.png

# Using Inkscape
inkscape --export-type=png --export-width=512 assets/icon-main.svg

# Using rsvg-convert
rsvg-convert -w 512 -h 512 assets/icon-main.svg -o assets/icon-main.png
```

### SVG to ICO (Favicon)
```bash
# Create multiple sizes
convert assets/icon-minimal.svg -define icon:auto-resize=16,32,48,64,256 favicon.ico
```

### SVG to Different Sizes
```bash
# 16x16 (favicon)
convert -background none -density 300 assets/icon-minimal.svg -resize 16x16 icon-16.png

# 32x32 (small icon)
convert -background none -density 300 assets/icon-minimal.svg -resize 32x32 icon-32.png

# 128x128 (medium)
convert -background none -density 300 assets/icon-main.svg -resize 128x128 icon-128.png

# 512x512 (high-res)
convert -background none -density 300 assets/icon-gradient.svg -resize 512x512 icon-512.png
```

---

## 🎯 Design Rationale

### Logo Elements

#### "NQ" Letters
- **Meaning**: Next-gen Query / No-overhead Quick
- **Style**: Bold, modern sans-serif
- **Color**: White for maximum contrast
- **Size**: Large and prominent (primary focus)

#### JSON Brackets "{ }"
- **Meaning**: JSON library identity
- **Style**: Monospace font (code reference)
- **Color**: Light cyan (subtle, supporting)
- **Position**: Below NQ, smaller size

#### Query Arrow
- **Meaning**: Path-based queries, direction, flow
- **Style**: Simple geometric arrow
- **Color**: Light cyan accent
- **Position**: Right side (forward motion)

#### Circuit Lines
- **Meaning**: Performance, optimization, technical sophistication
- **Style**: Simple connected lines with dots
- **Color**: Light cyan, low opacity
- **Position**: Bottom left (subtle detail)

#### Zero Allocation Badge (dark icon)
- **Meaning**: Zero-allocation performance
- **Style**: Circle badge with "0 alloc"
- **Color**: Deep blue background, white text
- **Position**: Bottom right corner

---

## 📱 Platform-Specific Guidelines

### GitHub
- **Profile Picture**: Use `icon-main.svg` or `icon-minimal.svg`
- **Social Preview**: Use `social-preview.svg` converted to PNG
- **README Header**: Use `icon-main.svg` at 150-200px width

### npm (if you create JS version)
- **Package Icon**: Convert `icon-minimal.svg` to PNG 512x512
- **Format**: PNG with transparent background

### Documentation Site
- **Logo**: Use `icon-gradient.svg` in hero section
- **Favicon**: Use `icon-minimal.svg`
- **Dark Mode**: Use `icon-dark.svg`

### Social Media
- **Twitter**: Use `social-preview.svg` (1200x630)
- **LinkedIn**: Use `social-preview.svg` (1200x630)
- **Profile Image**: Use `icon-minimal.svg` (circular crop)

---

## 🎨 Alternative Color Schemes (Optional)

### Dark Mode Variant
```css
--nqjson-bg-dark:     #1a1a1a
--nqjson-cyan-dark:   #00B4D8
--nqjson-text-dark:   #FFFFFF
--nqjson-accent-dark: #90E0EF
```

### High Contrast (Accessibility)
```css
--nqjson-high-contrast-bg:   #000000
--nqjson-high-contrast-fg:   #FFFFFF
--nqjson-high-contrast-link: #00D4FF
```

---

## ✅ Quick Start Checklist

For immediate use:

- [ ] Copy `icon-main.svg` to your repository
- [ ] Add icon to README.md header
- [ ] Convert `icon-minimal.svg` to PNG for favicon
- [ ] Upload `social-preview.svg` to GitHub social preview
- [ ] Use consistent brand colors across documentation
- [ ] Create favicon.ico for website (if applicable)

---

## 📐 Design Files

All icons are created as SVG (Scalable Vector Graphics):
- ✅ Infinite scalability (no quality loss)
- ✅ Small file size (typically < 10KB)
- ✅ Easy to edit in any vector editor
- ✅ Supports transparency
- ✅ Works on all modern browsers

### Editing Icons
You can edit these SVG files in:
- **Adobe Illustrator** (Professional)
- **Figma** (Web-based, free)
- **Inkscape** (Free, open-source)
- **Sketch** (Mac)
- **Any text editor** (SVG is XML)

---

## 🚀 Implementation Example

### In Your README.md
```markdown
<div align="center">
  <img src="assets/icon-main.svg" width="180" alt="nqjson logo"/>
  
  # nqjson
  
  **Next-gen Query JSON for Go**
  
  [![Go Reference](https://pkg.go.dev/badge/github.com/dhawalhost/nqjson.svg)](https://pkg.go.dev/github.com/dhawalhost/nqjson)
  [![Go Report Card](https://goreportcard.com/badge/github.com/dhawalhost/nqjson)](https://goreportcard.com/report/github.com/dhawalhost/nqjson)
  
  Fast JSON operations with zero allocations
</div>
```

### In Your Website
```html
<!DOCTYPE html>
<html>
<head>
  <title>nqjson - Next-gen Query JSON</title>
  <link rel="icon" type="image/svg+xml" href="/assets/icon-minimal.svg">
  <meta property="og:image" content="https://yourdomain.com/assets/social-preview.png">
</head>
<body>
  <header>
    <img src="/assets/icon-gradient.svg" width="100" alt="nqjson">
    <h1>nqjson</h1>
  </header>
</body>
</html>
```

---

## 📝 License

These icon designs are part of the nqjson project and follow the same MIT License.

You are free to:
- ✅ Use in personal and commercial projects
- ✅ Modify colors and sizes
- ✅ Create derivative works
- ✅ Include in documentation

---

## 🎉 Summary

**5 icon variants created:**
1. ✅ Main icon (icon-main.svg) - Recommended default
2. ✅ Gradient icon (icon-gradient.svg) - Premium look
3. ✅ Dark icon (icon-dark.svg) - Dark theme
4. ✅ Minimal icon (icon-minimal.svg) - Small sizes
5. ✅ Social preview (social-preview.svg) - Sharing

**Brand identity established:**
- Clear color palette (Cyan + Blue)
- Consistent typography (Bold sans-serif)
- Meaningful visual elements (NQ, arrows, brackets)
- Professional and modern aesthetic

**Ready to use immediately!** 🚀

---

*Icon assets created: October 17, 2025*  
*For: nqjson - Next-gen Query JSON for Go*
