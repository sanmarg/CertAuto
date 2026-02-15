/* ============================================
   CertAuto - Advanced Animations
   Scroll Effects & Parallax
   ============================================ */

// Parallax effect for hero section
const heroVisual = document.querySelector('.hero-visual');
if (heroVisual) {
    window.addEventListener('scroll', () => {
        const scrollPosition = window.pageYOffset;
        const heroSection = document.querySelector('.hero');
        
        if (heroSection && scrollPosition < heroSection.offsetHeight) {
            const offset = scrollPosition * 0.5;
            heroVisual.style.transform = `translateY(${offset}px)`;
        }
    });
}

// Animate number counters
function animateCounter(element, target, duration = 2000) {
    const start = 0;
    const increment = target / (duration / 50);
    let current = start;

    const counter = setInterval(() => {
        current += increment;
        if (current >= target) {
            element.textContent = target + (element.textContent.match(/\D/)?.[0] || '');
            clearInterval(counter);
        } else {
            element.textContent = Math.floor(current) + (element.textContent.match(/\D/)?.[0] || '');
        }
    }, 50);
}

// Observe stat elements and animate when in view
const statObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        if (entry.isIntersecting && !entry.target.dataset.animated) {
            entry.target.dataset.animated = 'true';
            
            const statNumbers = entry.target.querySelectorAll('.stat-number');
            statNumbers.forEach(stat => {
                const text = stat.textContent;
                const number = parseInt(text);
                if (!isNaN(number)) {
                    animateCounter(stat, number);
                }
            });
            
            statObserver.unobserve(entry.target);
        }
    });
}, { threshold: 0.5 });

const heroStats = document.querySelector('.hero-stats');
if (heroStats) {
    statObserver.observe(heroStats);
}

// Stagger animation for cards
function createStaggerAnimation(selector, delay = 100) {
    const elements = document.querySelectorAll(selector);
    let index = 0;

    const observer = new IntersectionObserver((entries) => {
        entries.forEach(entry => {
            if (entry.isIntersecting) {
                entry.target.style.animation = `slideInUp 0.6s ease-out forwards`;
                entry.target.style.animationDelay = `${index * delay}ms`;
                index++;
                observer.unobserve(entry.target);
            }
        });
    }, { threshold: 0.1 });

    elements.forEach(el => {
        el.style.opacity = '0';
        observer.observe(el);
    });
}

// Apply stagger animations to cards
createStaggerAnimation('.feature-card', 80);
createStaggerAnimation('.doc-card', 50);
createStaggerAnimation('.tech-item', 60);

// Text reveal animation
function revealText(element) {
    const text = element.textContent;
    const textArray = text.split('');
    
    element.style.minHeight = element.offsetHeight + 'px';
    element.textContent = '';

    textArray.forEach((char, index) => {
        const span = document.createElement('span');
        span.textContent = char;
        span.style.opacity = '0';
        span.style.animation = `fadeIn 0.05s ease-in forwards`;
        span.style.animationDelay = `${index * 30}ms`;
        element.appendChild(span);
    });
}

// Heading animations
const headings = document.querySelectorAll('h1, h2, h3');
const headingObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        if (entry.isIntersecting && !entry.target.dataset.revealed) {
            entry.target.dataset.revealed = 'true';
            // Apply slide-in animation instead of letter animation for performance
            entry.target.style.animation = 'slideInUp 0.6s ease-out forwards';
            headingObserver.unobserve(entry.target);
        }
    });
}, { threshold: 0.5 });

headings.forEach(heading => {
    heading.style.opacity = '0';
    headingObserver.observe(heading);
});

// Gradient animation for gradient text
function animateGradient(element) {
    const angle = Math.random() * 360;
    element.style.backgroundImage = `linear-gradient(${angle}deg, #6366f1, #a855f7)`;
    element.style.animation = 'gradientShift 3s ease-in-out infinite';
}

document.querySelectorAll('.gradient-text, .gradient-text-alt').forEach(el => {
    // Optional: uncomment to enable gradient animation
    // animateGradient(el);
});

// Scroll-triggered animations for specific elements
const triggerObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        if (entry.isIntersecting) {
            // Add animation class
            entry.target.classList.add('animated');
            
            // Optional: Add glow effect to feature cards
            if (entry.target.classList.contains('feature-card')) {
                entry.target.style.boxShadow = 
                    'inset 0 0 20px rgba(99, 102, 241, 0.1), 0 10px 15px -3px rgba(99, 102, 241, 0.1)';
            }
        }
    });
}, { threshold: 0.1, rootMargin: '0px 0px -50px 0px' });

document.querySelectorAll('.feature-card, .step-content, .doc-card').forEach(el => {
    triggerObserver.observe(el);
});

// Parallax effect for background elements
function setupParallax() {
    const parallaxElements = document.querySelectorAll('[data-parallax]');
    
    window.addEventListener('scroll', () => {
        parallaxElements.forEach(element => {
            const speed = element.getAttribute('data-parallax');
            const yPos = window.pageYOffset * speed;
            element.style.transform = `translateY(${ypos}px)`;
        });
    });
}

// Optional parallax for hero background blobs
const blobs = document.querySelectorAll('.gradient-blob');
window.addEventListener('scroll', () => {
    const scrolled = window.pageYOffset;
    blobs.forEach((blob, index) => {
        const speed = 0.3 + (index * 0.1);
        blob.style.transform = `translateY(${scrolled * speed}px)`;
    });
});

// Mouse follow effect (optional - subtle)
let mouseX = 0;
let mouseY = 0;

document.addEventListener('mousemove', (e) => {
    mouseX = e.clientX / window.innerWidth;
    mouseY = e.clientY / window.innerHeight;
    
    // Apply subtle effect to blobs on hero section
    const heroSection = document.querySelector('.hero');
    if (heroSection && window.pageYOffset < heroSection.offsetHeight) {
        const blobs = document.querySelectorAll('.gradient-blob');
        blobs.forEach((blob, index) => {
            const moveX = (mouseX - 0.5) * 20 * (index + 1);
            const moveY = (mouseY - 0.5) * 20 * (index + 1);
            blob.style.transform = `
                translate(${moveX}px, ${moveY}px)
                ${window.pageYOffset && `translateY(${window.pageYOffset * (0.3 + index * 0.1)}px)`}
            `;
        });
    }
});

// Smooth number formatting for stats
function formatNumber(num) {
    if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M';
    if (num >= 1000) return (num / 1000).toFixed(1) + 'K';
    return num.toString();
}

// Dynamic background coloring on scroll
window.addEventListener('scroll', () => {
    const scrollPercentage = (window.scrollY / (document.documentElement.scrollHeight - window.innerHeight)) * 100;
    
    // Optional: Apply subtle background shifts
    // Can be enabled for more dynamic effects
});

// Cursor effect (optional - uncomment to enable)
/*
const cursor = document.createElement('div');
cursor.className = 'custom-cursor';
document.body.appendChild(cursor);

document.addEventListener('mousemove', (e) => {
    cursor.style.left = e.clientX + 'px';
    cursor.style.top = e.clientY + 'px';
});

document.addEventListener('mousedown', () => {
    cursor.classList.add('active');
});

document.addEventListener('mouseup', () => {
    cursor.classList.remove('active');
});
*/

// Page visibility optimization
document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
        // Pause animations if tab is not visible
        document.querySelectorAll('*').forEach(el => {
            el.style.animationPlayState = 'paused';
        });
    } else {
        // Resume animations
        document.querySelectorAll('*').forEach(el => {
            el.style.animationPlayState = 'running';
        });
    }
});

// Scroll to top button (can be implemented)
const createScrollToTopBtn = () => {
    const btn = document.createElement('button');
    btn.className = 'scroll-to-top';
    btn.innerHTML = '↑';
    btn.style.cssText = `
        position: fixed;
        bottom: 2rem;
        right: 2rem;
        width: 50px;
        height: 50px;
        background: linear-gradient(135deg, #6366f1, #a855f7);
        color: white;
        border: none;
        border-radius: 50%;
        cursor: pointer;
        opacity: 0;
        transition: opacity 0.3s, transform 0.3s;
        z-index: 999;
        font-size: 1.5rem;
        display: flex;
        align-items: center;
        justify-content: center;
        box-shadow: 0 4px 15px rgba(99, 102, 241, 0.3);
    `;

    window.addEventListener('scroll', () => {
        if (window.pageYOffset > 300) {
            btn.style.opacity = '1';
            btn.style.pointerEvents = 'auto';
        } else {
            btn.style.opacity = '0';
            btn.style.pointerEvents = 'none';
        }
    });

    btn.addEventListener('click', () => {
        window.scrollTo({ top: 0, behavior: 'smooth' });
    });

    btn.addEventListener('mouseover', () => {
        btn.style.transform = 'scale(1.1)';
    });

    btn.addEventListener('mouseout', () => {
        btn.style.transform = 'scale(1)';
    });

    document.body.appendChild(btn);
};

// Initialize scroll to top button
createScrollToTopBtn();

// Performance: Use requestAnimationFrame for smooth animations
function smoothAnimationFrame() {
    requestAnimationFrame(smoothAnimationFrame);
    
    // Update animations here if needed
}

smoothAnimationFrame();

console.log('Advanced animations loaded! 🎨');
