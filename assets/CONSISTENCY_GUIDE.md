# Icon Consistency & Scaling Strategy

**Updated**: October 17, 2025  
**Project**: nqjson - Next-gen Query JSON for Go

---

## 🎯 Design Principle: Progressive Detail

All icons now follow a **unified visual identity** with consistent typography and progressive detail based on size/context.

---

## 📐 Unified Design System

### Core Elements (Present in ALL versions)

1. **"NQ" Letters**
   - Font: `system-ui, -apple-system, sans-serif` (consistent across ALL icons)
   - Weight: `900` (extra bold)
   - Color: `#FFFFFF` (white)
   - Letter-spacing: `-3` to `-8` (tight, modern)
   - Position: Center, prominent

2. **"json" Subtext**
   - Font: `system-ui, -apple-system, sans-serif` (SAME font family)
   - Weight: `600` (semi-bold)
   - Color: `#FFFFFF` with 80-85% opacity
   - Size: ~26% of NQ size
   - Position: Below NQ

3. **Brand Colors** (consistent everywhere)
   - Primary: `#00B4D8` (Cyan)
   - Secondary: `#0077B6` (Deep Blue)
   - Accent: `#90E0EF` (Light Cyan)

---

## 📊 Progressive Complexity by Size

### **Minimal (200x200)** - CORE ONLY
```
Elements: 2
├── NQ letters (85px, font-weight: 900)
└── Solid cyan background

Purpose: Favicons, tiny thumbnails
Philosophy: Maximum clarity at small sizes
```

### **Main (512x512)** - STANDARD
```
Elements: 4
├── NQ letters (180px, font-weight: 900)
├── json subtext (48px, font-weight: 600)
├── Circle background with stroke
└── Decorative query path line

Purpose: GitHub profile, README, general use
Philosophy: Recognizable with subtle detail
```

### **Gradient (512x512)** - ENHANCED
```
Elements: 6
├── NQ letters (180px, font-weight: 900) - SAME
├── json subtext (48px, font-weight: 600) - SAME
├── Gradient background (rounded square)
├── Decorative query path line - SAME
└── Speed lines (3 lines) - ADDITIONAL DETAIL

Purpose: Marketing, presentations, hero sections
Philosophy: Premium look with extra visual interest
```

### **Dark (512x512)** - THEMED
```
Elements: 6
├── NQ letters (180px, font-weight: 900) - SAME + glow
├── json subtext (48px, font-weight: 600) - SAME
├── Dark background (#1a1a1a)
├── Decorative query path line - SAME
└── Corner accents (4 dots) - ADDITIONAL DETAIL

Purpose: Dark mode interfaces, technical contexts
Philosophy: Same core, adapted for dark backgrounds
```

### **Social (1200x630)** - COMPLETE
```
Elements: 8
├── NQ letters (220px, font-weight: 900) - SAME font, scaled
├── json subtext (58px, font-weight: 600) - SAME font, scaled
├── Tagline (36px, font-weight: 400) - SAME font family
├── Gradient background
├── Grid pattern (subtle)
├── Decorative query path line - SAME style
└── Feature badges (3) - SAME font

Purpose: Social sharing, GitHub preview
Philosophy: Maximum context and information
```

---

## 🎨 Typography Specification

### Font Stack (Used Everywhere)
```css
font-family: system-ui, -apple-system, sans-serif
```

**Why this choice:**
- ✅ Native system font (instant loading)
- ✅ Consistent across all platforms
- ✅ Modern, clean appearance
- ✅ Excellent readability at all sizes
- ✅ Professional and technical feel

### Font Weights
```css
NQ letters:   900 (extra bold) - Maximum impact
json text:    600 (semi-bold)  - Clear but subtle
tagline:      400 (regular)    - Easy to read
badges:       600 (semi-bold)  - Professional
```

### Size Ratios (Consistent)
```
NQ size:      Base (100%)
json size:    26.7% of NQ
tagline:      20% of NQ (social only)
badges:       10% of NQ (social only)
```

---

## 🎯 Visual Consistency Rules

### Rule 1: Same Core, Different Detail
- **Core elements** (NQ + json) are identical across all icons
- **Additional elements** are added progressively based on size/purpose
- Typography is 100% consistent

### Rule 2: Proportional Scaling
```
Minimal:  NQ=85px,  json=none    (smallest)
Main:     NQ=180px, json=48px    (standard)
Gradient: NQ=180px, json=48px    (same as main)
Dark:     NQ=180px, json=48px    (same as main)
Social:   NQ=220px, json=58px    (largest)
```

### Rule 3: Detail Budget
```
Minimal:   2 elements (just the essentials)
Main:      4 elements (add structure)
Gradient:  6 elements (add visual interest)
Dark:      6 elements (add atmosphere)
Social:    8 elements (add context)
```

---

## 📱 Recognition Test

### At 16x16 (Favicon)
```
✅ "NQ" is clearly visible
✅ Colors are recognizable
✅ No clutter or confusion
```

### At 128x128 (App Icon)
```
✅ "NQ" + "json" are clear
✅ Path line adds interest
✅ Professional appearance
```

### At 512x512 (Profile)
```
✅ Full detail visible
✅ Subtle decorations enhance
✅ Brand identity strong
```

### At 1200x630 (Social)
```
✅ Complete information
✅ Context and features clear
✅ Compelling presentation
```

---

## 🔄 Scaling Examples

### Minimal → Main (Scale Up)
```
ADD:  "json" text
ADD:  Query path line
ADD:  Background circle with stroke
```

### Main → Gradient (Same Size, More Detail)
```
KEEP: NQ + json (identical)
KEEP: Query path line (identical)
CHANGE: Background (solid → gradient)
ADD:  Speed lines (3)
```

### Main → Dark (Same Size, Different Theme)
```
KEEP: NQ + json (identical font/size)
KEEP: Query path line (identical)
CHANGE: Background (cyan → dark)
CHANGE: NQ color (white → cyan with glow)
ADD:  Corner accents (4 dots)
```

### Main → Social (Scale Up + Context)
```
SCALE UP: NQ + json (proportional)
KEEP: Same fonts and weights
ADD:  Tagline
ADD:  Feature badges
ADD:  Grid pattern
```

---

## ✅ Consistency Checklist

Use this to verify all icons maintain consistency:

### Typography
- [ ] All use `system-ui, -apple-system, sans-serif`
- [ ] NQ is always weight `900`
- [ ] json is always weight `600`
- [ ] Letter-spacing is consistent for size

### Colors
- [ ] Primary cyan: `#00B4D8`
- [ ] Secondary blue: `#0077B6`
- [ ] Accent cyan: `#90E0EF`
- [ ] White text: `#FFFFFF`

### Proportions
- [ ] json text is ~27% of NQ size
- [ ] Query path line has same style
- [ ] Spacing is proportional to size

### Core Elements
- [ ] NQ letters present in all versions
- [ ] json text present in all except minimal
- [ ] Same letter-spacing ratio
- [ ] Centered alignment

---

## 🎨 Before vs After

### Before (Issues)
```
❌ Different fonts (Arial, monospace, mixed)
❌ Inconsistent weights (bold vs 600 vs 900)
❌ Different visual styles
❌ Hard to recognize across versions
❌ No clear scaling strategy
```

### After (Unified)
```
✅ Single font family (system-ui)
✅ Consistent weights (900/600/400)
✅ Same core design everywhere
✅ Instantly recognizable
✅ Progressive detail scaling
```

---

## 💡 Design Philosophy

### Minimal Viable Icon
At the smallest size, we show **only what matters**:
- The "NQ" mark
- The brand color

### Standard Icon
At typical sizes, we add **structure and identity**:
- The "NQ" mark
- The "json" context
- A subtle decorative element

### Enhanced Icons
At larger sizes or special contexts, we add **atmosphere and detail**:
- All standard elements
- Visual effects (gradients, glows)
- Additional decorative elements
- Contextual information (taglines, badges)

---

## 🚀 Usage Guidelines

### When to Use Each Version

| Icon | Best For | Why |
|------|----------|-----|
| **Minimal** | Favicon, 16-32px | Maximum clarity, no detail needed |
| **Main** | README, profile, docs | Standard version, balanced |
| **Gradient** | Hero sections, marketing | Premium feel, eye-catching |
| **Dark** | Dark mode sites | Optimized for dark backgrounds |
| **Social** | Sharing, previews | Maximum context and info |

### Scaling Guidelines

```
16px-64px:    Use minimal
64px-256px:   Use main or gradient
256px-512px:  Use main, gradient, or dark
1200x630:     Use social
```

---

## ✅ Result

**One unified brand identity** that:
- Scales from 16px to 1200px
- Maintains consistency across all contexts
- Uses the same typography everywhere
- Adds detail progressively as needed
- Is instantly recognizable at any size

---

*Updated: October 17, 2025*  
*All icons now follow unified design system*
