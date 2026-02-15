# CertAuto Website - Deployment & Maintenance Guide

## Quick Start

### Prerequisites
- Git installed
- GitHub account with repository access
- (Optional) Local web server for testing

### Local Development

1. **Clone the repository:**
   ```bash
   git clone https://github.com/sanmarg/CertAuto.git
   cd certauto
   ```

2. **Serve locally (choose one):**

   **Python 3:**
   ```bash
   cd docs
   python -m http.server 8000
   # Open http://localhost:8000 in browser
   ```

   **Python 2:**
   ```bash
   cd docs
   python -m SimpleHTTPServer 8000
   ```

   **Node.js:**
   ```bash
   cd docs
   npx http-server
   ```

   **Using Live Server Extension (VS Code):**
   - Install "Live Server" extension
   - Right-click `index.html` → "Open with Live Server"

## Deployment

### Automatic Deployment (GitHub Pages)

The website automatically deploys when you push to the repository:

1. **Push changes to main/master branch:**
   ```bash
   git add docs/
   git commit -m "Update website content"
   git push origin main
   ```

2. **GitHub Actions automatically:**
   - Validates the HTML
   - Checks for broken links
   - Uploads files to GitHub Pages
   - Website updates at `https://sanmarg.github.io/certauto`

3. **Monitor deployment:**
   - Go to repository → Actions tab
   - Check the latest workflow run
   - Look for "deploy-website.yml" status

### Manual Verification

After deployment, verify:

1. Visit: `https://sanmarg.github.io/certauto`
2. Check console (F12) for errors
3. Test responsive design (mobile view)
4. Verify all links are working

## Content Updates

### Update Project Information

**File: `docs/index.html`**
- Edit section content
- Update contributor information
- Modify feature descriptions
- Add new sections

Example - Update a feature:
```html
<div class="feature-card">
    <div class="feature-icon"><!-- icon --></div>
    <h3>New Feature Title</h3>
    <p>New feature description here</p>
</div>
```

### Add New Sections

To add a new section to the website:

1. Add HTML in `index.html`:
   ```html
   <section id="my-section" class="my-section">
       <div class="container">
           <!-- Content -->
       </div>
   </section>
   ```

2. Add styles in `css/main.css`:
   ```css
   .my-section {
       background: var(--bg-primary);
       padding: var(--space-3xl) var(--space-xl);
   }
   ```

3. Add navigation link:
   ```html
   <li><a href="#my-section" class="nav-link">My Section</a></li>
   ```

### Customize Colors

**File: `css/main.css`**

Update CSS variables:
```css
:root {
    --primary-color: #6366f1;    /* Change primary color */
    --secondary-color: #a855f7;  /* Change secondary color */
    --accent-color: #ec4899;     /* Change accent color */
    /* ... other colors ... */
}
```

### Update Animations

**File: `css/animations.css`**

Modify animation keyframes:
```css
@keyframes slideInUp {
    from {
        opacity: 0;
        transform: translateY(30px);
    }
    to {
        opacity: 1;
        transform: translateY(0);
    }
}
```

## Performance Optimization

### Image & Asset Guidelines

1. **Minimize file sizes:**
   - Use SVG for icons/logos
   - Compress images to < 100KB
   - Use WebP format when possible

2. **Lazy loading:**
   - Images below fold are lazy-loaded automatically

3. **Caching:**
   - Service Worker caches assets
   - Browser cache controlled by headers

### Testing Performance

1. **Google Lighthouse:**
   - Open DevTools (F12) → Lighthouse tab
   - Run audit
   - Target: >90 for all metrics

2. **Performance targets:**
   - First Contentful Paint: < 1s
   - Largest Contentful Paint: < 2.5s
   - Cumulative Layout Shift: < 0.1
   - Time to Interactive: < 3s

## Maintenance Tasks

### Weekly
- Monitor GitHub Actions for failed deployments
- Check broken links occasionally
- Review user feedback/issues

### Monthly
- Update contributor information
- Refresh feature descriptions
- Check for outdated documentation links

### Quarterly
- Review analytics (if enabled)
- Optimize performance
- Update technology stack versions
- Refresh design if needed

## Troubleshooting

### Website not updating
1. Check GitHub Actions status
2. Verify files are in `/docs` directory
3. Clear browser cache (Ctrl+Shift+Delete)
4. Wait 1-2 minutes for GitHub Pages rebuild

### Links broken
1. Check file paths are correct
2. Verify anchors match section IDs
3. External links use full URLs
4. Run broken link checker tool

### Styling not applied
1. Clear browser cache
2. Check CSS file paths
3. Verify CSS syntax (no typos)
4. Check CSS variable definitions
5. Ensure CSS is linked in HTML

### JavaScript errors
1. Open browser console (F12)
2. Check error messages
3. Verify all script sources exist
4. Check for syntax errors

## Security

### Best Practices
- ✅ No sensitive information in code
- ✅ Use HTTPS only (enforced by GitHub)
- ✅ Validate all external links
- ✅ Keep dependencies updated
- ✅ Monitor for security alerts

### Configure Security Settings
1. GitHub repo → Settings → Security
2. Enable branch protection rules
3. Require code review for PRs
4. Require status checks to pass

## SEO Configuration

### Metadata
Edit in `index.html` head section:
```html
<meta name="description" content="...">
<meta name="keywords" content="...">
<meta property="og:title" content="...">
```

### Structured Data
Add JSON-LD in `index.html`:
```json
{
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  "name": "CertAuto"
}
```

### Sitemap & Robots
- `sitemap.xml` - List of all pages
- `robots.txt` - Search engine directives

## Analytics (Optional)

To add Google Analytics:

1. Get GA4 property ID
2. Add to `_config.yml`:
   ```yaml
   google_analytics: "G-YOUR-ID"
   ```
3. Or add to `index.html`:
   ```html
   <script async src="https://www.googletagmanager.com/gtag/js?id=G-YOUR-ID"></script>
   <script>
     window.dataLayer = window.dataLayer || [];
     function gtag(){dataLayer.push(arguments);}
     gtag('js', new Date());
     gtag('config', 'G-YOUR-ID');
   </script>
   ```

## File Structure Reference

```
docs/
├── index.html              # Main page
├── css/
│   ├── main.css           # Primary styles
│   └── animations.css     # Animations
├── js/
│   ├── main.js            # Main functionality
│   ├── animations.js      # Advanced animations
│   └── sw.js              # Service Worker
├── assets/                # Images/media
│   └── (add images here)
├── manifest.json          # PWA manifest
├── sitemap.xml            # SEO sitemap
├── robots.txt             # SEO directives
├── browserconfig.xml      # Windows config
├── opensearch.xml         # Search integration
├── .htaccess              # Apache config (optional)
├── _config.yml            # Jekyll config
├── .nojekyll              # Disable Jekyll
└── README.md              # This guide
```

## Contributing Guidelines

When updating the website:

1. Create a branch: `git checkout -b improve/website-section`
2. Make changes in `/docs` directory
3. Test locally
4. Commit with clear message: `git commit -m "Improve: update features section"`
5. Push: `git push origin improve/website-section`
6. Create Pull Request with description

## Deployment Checklist

Before publishing changes:
- [ ] Test locally in multiple browsers
- [ ] Verify responsive design (mobile, tablet, desktop)
- [ ] Check all links work
- [ ] Test navigation
- [ ] Verify animations work
- [ ] Check console for errors (F12)
- [ ] Optimize images
- [ ] Review content for typos

## Quick Commands

```bash
# View local site
python -m http.server 8000

# Commit and push changes
git add docs/
git commit -m "Update: website content"
git push origin main

# Check deployment status
# Go to: https://github.com/sanmarg/CertAuto/actions

# View live site
# Visit: https://sanmarg.github.io/certauto
```

## Support

For issues or questions:
- Check GitHub Issues: https://github.com/sanmarg/CertAuto/issues
- Create new issue with detailed information
- Follow issue template
- Provide screenshots when possible

## Resources

- [GitHub Pages Documentation](https://docs.github.com/en/pages)
- [HTML5 Specifications](https://html.spec.whatwg.org/)
- [CSS Reference](https://developer.mozilla.org/en-US/docs/Web/CSS)
- [JavaScript Guide](https://developer.mozilla.org/en-US/docs/Web/JavaScript)
- [Web Performance Guide](https://web.dev/performance/)
- [Accessibility Guidelines](https://www.w3.org/WAI/WCAG21/quickref/)

---

**Last Updated:** February 2025
**Maintained by:** Sanmarg Paranjpe
