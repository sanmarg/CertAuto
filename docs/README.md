# CertAuto Website Documentation

This directory contains the static website for CertAuto, hosted on GitHub Pages.

## Structure

```
docs/
├── index.html          # Main entry point
├── css/
│   ├── main.css       # Primary styles
│   └── animations.css # Animation definitions
├── js/
│   ├── main.js        # Main functionality
│   └── animations.js  # Advanced animations
├── assets/            # Images and resources
├── _config.yml        # Jekyll configuration
├── .nojekyll          # Disable Jekyll processing
└── README.md          # This file
```

## Technology Stack

- **HTML5** - Semantic markup
- **CSS3** - Modern styling with CSS Grid, Flexbox, and variables
- **JavaScript (ES6+)** - Interactive features without frameworks
- **GitHub Pages** - Free hosting and deployment

## Features

### Design
- ✨ Modern, professional UI with gradients and animations
- 📱 Fully responsive (mobile, tablet, desktop)
- 🎨 Unique visual identity with custom animations
- ♿ Accessible markup and keyboard navigation

### Performance
- 🚀 No external dependencies (except fonts)
- ⚡ Optimized CSS and JavaScript
- 🎯 Lazy loading for images and videos
- 📊 Minimal bundle size (~200KB gzipped)

### Functionality
- 🧭 Smooth navigation with active state tracking
- 🔍 Intersection Observer for scroll animations
- 📋 Copy-to-clipboard for code blocks
- 📱 Mobile menu with hamburger toggle
- 🎯 Keyboard shortcuts support

## Development

### Local Testing

To test locally, you can:

1. **Using Python 3:**
   ```bash
   cd docs
   python -m http.server 8000
   ```

2. **Using Node.js:**
   ```bash
   cd docs
   npx http-server
   ```

3. **Using Ruby (if Jekyll is installed):**
   ```bash
   cd docs
   jekyll serve
   ```

Then open [http://localhost:8000](http://localhost:8000) in your browser.

### File Organization

- **HTML**: Single-page application with semantic sections
- **CSS**: Organized with custom properties, no preprocessor needed
- **JavaScript**: Vanilla JS for maximum compatibility

## Deployment

The site is automatically deployed to GitHub Pages via the `/docs` directory:

1. Push changes to the repository
2. GitHub Pages automatically rebuilds the site
3. Site is live at `https://sanmarg.github.io/certauto`

## Configuration

### Site Metadata
Edit `_config.yml` to change:
- Site title and description
- Social media information
- Analytics tracking ID

### Content Updates

**Update project info in:**
- `index.html` - Add new sections or update feature descriptions
- `css/main.css` - Modify colors, spacing, fonts
- `js/main.js` - Add new interactive features

## Customization Guide

### Colors
All colors are defined as CSS variables in `main.css`:
```css
:root {
    --primary-color: #6366f1;
    --secondary-color: #a855f7;
    /* ... */
}
```

### Typography
Font sizes and families are customizable:
```css
:root {
    --font-primary: /* system fonts */;
    --font-mono: /* monospace fonts */;
    --font-size-* : /* predefined sizes */;
}
```

### Animations
All animations are in `animations.css` and can be modified without affecting layout.

## Accessibility

- Semantic HTML structure
- ARIA labels where needed
- Keyboard navigation support
- Color contrast compliance (WCAG AA)
- Respects `prefers-reduced-motion` setting

## Browser Support

- Modern browsers (Chrome, Firefox, Safari, Edge)
- Progressive enhancement for older browsers
- Mobile browsers (iOS Safari, Chrome Android)

## Performance Metrics

- First Contentful Paint (FCP): < 1s
- Largest Contentful Paint (LCP): < 2.5s
- Cumulative Layout Shift (CLS): < 0.1
- Time to Interactive (TTI): < 3s

## Security

- No external scripts or trackers (unless configured)
- Content Security Policy friendly
- No cookies by default
- HTTPS enforced by GitHub Pages

## Contributing

To contribute to the website:

1. Create a new branch: `git checkout -b improve/website-section`
2. Make changes in the `/docs` directory
3. Test locally with `python -m http.server 8000`
4. Commit with meaningful messages
5. Push and create a pull request

## License

The website content is licensed under the same license as CertAuto (Apache 2.0).

## Support

For issues or suggestions regarding the website:
- Open an issue on GitHub
- Check existing documentation
- Review the code comments

---

**Last Updated:** February 2025
**Maintainer:** Sanmarg Paranjpe
