/* ============================================
   CertAuto - Main JavaScript
   Interactive Features & DOM Manipulation
   ============================================ */

// Mobile Navigation Toggle
const mobileToggle = document.getElementById('mobileToggle');
const navMenu = document.getElementById('navMenu');
const navLinks = document.querySelectorAll('.nav-link');

if (mobileToggle && navMenu) {
    mobileToggle.addEventListener('click', () => {
        navMenu.classList.toggle('active');
        
        // Animate hamburger
        const spans = mobileToggle.querySelectorAll('span');
        if (navMenu.classList.contains('active')) {
            spans[0].style.transform = 'rotate(45deg) translateY(8px)';
            spans[1].style.opacity = '0';
            spans[2].style.transform = 'rotate(-45deg) translateY(-8px)';
        } else {
            spans[0].style.transform = '';
            spans[1].style.opacity = '1';
            spans[2].style.transform = '';
        }
    });

    // Close menu when a link is clicked
    navLinks.forEach(link => {
        link.addEventListener('click', () => {
            navMenu.classList.remove('active');
            const spans = mobileToggle.querySelectorAll('span');
            spans[0].style.transform = '';
            spans[1].style.opacity = '1';
            spans[2].style.transform = '';
        });
    });
}

// Smooth scroll for anchor links
document.querySelectorAll('a[href^="#"]').forEach(anchor => {
    anchor.addEventListener('click', function (e) {
        e.preventDefault();
        const target = document.querySelector(this.getAttribute('href'));
        if (target) {
            const offset = 70; // Navbar height
            const targetPosition = target.offsetTop - offset;
            window.scrollTo({
                top: targetPosition,
                behavior: 'smooth'
            });
        }
    });
});

// Intersection Observer for scroll animations
const observerOptions = {
    threshold: 0.1,
    rootMargin: '0px 0px -50px 0px'
};

const observer = new IntersectionObserver((entries) => {
    entries.forEach((entry, index) => {
        if (entry.isIntersecting) {
            entry.target.classList.add('fade-in');
            entry.target.style.animationDelay = `${index * 0.1}s`;
            observer.unobserve(entry.target);
        }
    });
}, observerOptions);

// Observe elements that should animate in
document.querySelectorAll('.feature-card, .doc-card, .contributor-card, .tech-item').forEach(el => {
    observer.observe(el);
});

// Active nav link highlighting
window.addEventListener('scroll', () => {
    let current = '';
    const sections = document.querySelectorAll('section');

    sections.forEach(section => {
        const sectionTop = section.offsetTop;
        const sectionHeight = section.clientHeight;
        if (pageYOffset >= sectionTop - 200) {
            current = section.getAttribute('id');
        }
    });

    navLinks.forEach(link => {
        link.classList.remove('active');
        if (link.getAttribute('href').slice(1) === current) {
            link.style.color = '#6366f1';
        } else {
            link.style.color = '';
        }
    });
});

// Navbar background on scroll
window.addEventListener('scroll', () => {
    const navbar = document.querySelector('.navbar');
    if (window.pageYOffset > 50) {
        navbar.style.background = 'rgba(255, 255, 255, 0.98)';
        navbar.style.boxShadow = '0 4px 6px -1px rgba(0, 0, 0, 0.1)';
    } else {
        navbar.style.background = 'rgba(255, 255, 255, 0.95)';
        navbar.style.boxShadow = '0 1px 2px 0 rgba(0, 0, 0, 0.05)';
    }
});

// Code block copy functionality
document.querySelectorAll('pre').forEach(pre => {
    const codeBlock = pre.querySelector('code');
    if (codeBlock) {
        // Create copy button
        const copyBtn = document.createElement('button');
        copyBtn.className = 'copy-btn';
        copyBtn.innerHTML = 'Copy';
        copyBtn.style.cssText = `
            position: absolute;
            top: 10px;
            right: 10px;
            background: #6366f1;
            color: white;
            border: none;
            padding: 0.5rem 1rem;
            border-radius: 0.25rem;
            cursor: pointer;
            font-size: 0.875rem;
            transition: background 0.3s;
        `;

        pre.style.position = 'relative';
        pre.appendChild(copyBtn);

        copyBtn.addEventListener('mouseover', () => {
            copyBtn.style.background = '#4f46e5';
        });

        copyBtn.addEventListener('mouseout', () => {
            copyBtn.style.background = '#6366f1';
        });

        copyBtn.addEventListener('click', () => {
            const text = codeBlock.innerText;
            navigator.clipboard.writeText(text).then(() => {
                copyBtn.innerHTML = 'Copied!';
                setTimeout(() => {
                    copyBtn.innerHTML = 'Copy';
                }, 2000);
            });
        });
    }
});

// Video/Image lazy loading effect
const videoObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        if (entry.isIntersecting) {
            entry.target.style.opacity = '1';
            videoObserver.unobserve(entry.target);
        }
    });
}, { threshold: 0.1 });

document.querySelectorAll('img, video, .hero-visual').forEach(media => {
    media.style.opacity = '0';
    media.style.transition = 'opacity 0.6s ease-in-out';
    videoObserver.observe(media);
});

// Keyboard navigation
document.addEventListener('keydown', (e) => {
    // Press '?' or 'h' to show help
    if (e.key === '?' || (e.key === 'h' && !e.metaKey && !e.ctrlKey)) {
        console.log(`
CertAuto Website Keyboard Shortcuts:
- Press # or Directly click anchors to navigate to sections
- Press / to focus search (when available)
        `);
    }
});

// Detect dark mode preference
const prefersDark = window.matchMedia('(prefers-color-scheme: dark)');
if (prefersDark.matches) {
    // User prefers dark mode, but we're keeping light theme by design
    // This is here for future dark mode implementation
}

// Add ripple effect to buttons
document.querySelectorAll('.btn').forEach(button => {
    button.addEventListener('click', function(e) {
        const ripple = document.createElement('span');
        const rect = this.getBoundingClientRect();
        const size = Math.max(rect.width, rect.height);
        const x = e.clientX - rect.left - size / 2;
        const y = e.clientY - rect.top - size / 2;

        ripple.style.cssText = `
            position: absolute;
            width: ${size}px;
            height: ${size}px;
            border-radius: 50%;
            background: rgba(255, 255, 255, 0.7);
            left: ${x}px;
            top: ${y}px;
            animation: ripple-animation 0.6s ease-out;
            pointer-events: none;
        `;

        // Add ripple animation if not already present
        if (!document.querySelector('style[data-ripple]')) {
            const style = document.createElement('style');
            style.setAttribute('data-ripple', '');
            style.innerHTML = `
                @keyframes ripple-animation {
                    to {
                        transform: scale(4);
                        opacity: 0;
                    }
                }
            `;
            document.head.appendChild(style);
        }

        this.style.position = 'relative';
        this.style.overflow = 'hidden';
        this.appendChild(ripple);

        setTimeout(() => ripple.remove(), 600);
    });
});

// Performance monitoring (optional)
if (window.performance && window.performance.timing) {
    window.addEventListener('load', () => {
        setTimeout(() => {
            const perfData = window.performance.timing;
            const pageLoadTime = perfData.loadEventEnd - perfData.navigationStart;
            console.log(`Page load time: ${pageLoadTime}ms`);
        }, 0);
    });
}

// Prevent flash of unstyled content
document.documentElement.style.opacity = '1';

// Add loading state for external links
document.querySelectorAll('a[target="_blank"]').forEach(link => {
    link.addEventListener('click', function(e) {
        // Add any tracking or analytics here if needed
        console.log(`External link clicked: ${this.href}`);
    });
});

// Print friendly styles
const printStyle = document.createElement('style');
printStyle.innerHTML = `
    @media print {
        .navbar, .cta-section {
            display: none;
        }
        body {
            background: white;
        }
    }
`;
document.head.appendChild(printStyle);

console.log('CertAuto WebSite - All systems operational! 🚀');

// Service Worker Registration for PWA
if ('serviceWorker' in navigator) {
    navigator.serviceWorker.register('/certauto/js/sw.js')
        .then(registration => {
            console.log('Service Worker registered successfully:', registration);
        })
        .catch(error => {
            console.log('Service Worker registration failed:', error);
        });

    // Handle updates
    navigator.serviceWorker.addEventListener('controllerchange', () => {
        console.log('Service Worker updated');
    });
}
