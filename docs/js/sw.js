/* CertAuto - Service Worker
   Enables offline functionality and progressive web app capabilities
*/

const CACHE_NAME = 'certauto-v1';
const ASSETS_TO_CACHE = [
    '/certauto/',
    '/certauto/index.html',
    '/certauto/css/main.css',
    '/certauto/css/animations.css',
    '/certauto/js/main.js',
    '/certauto/js/animations.js',
    '/certauto/manifest.json',
    '/certauto/sitemap.xml'
];

// Install event - cache assets
self.addEventListener('install', event => {
    event.waitUntil(
        caches.open(CACHE_NAME).then(cache => {
            console.log('Caching assets');
            return cache.addAll(ASSETS_TO_CACHE).catch(err => {
                console.log('Cache addAll error:', err);
                // Continue even if some assets fail to cache
                return Promise.resolve();
            });
        })
    );
    self.skipWaiting();
});

// Activate event - clean up old caches
self.addEventListener('activate', event => {
    event.waitUntil(
        caches.keys().then(cacheNames => {
            return Promise.all(
                cacheNames.map(cacheName => {
                    if (cacheName !== CACHE_NAME) {
                        console.log('Deleting old cache:', cacheName);
                        return caches.delete(cacheName);
                    }
                })
            );
        })
    );
    self.clients.claim();
});

// Fetch event - serve from cache with network fallback
self.addEventListener('fetch', event => {
    // Skip non-GET requests
    if (event.request.method !== 'GET') {
        return;
    }

    event.respondWith(
        caches.match(event.request)
            .then(cachedResponse => {
                if (cachedResponse) {
                    return cachedResponse;
                }

                return fetch(event.request).then(response => {
                    // Don't cache non-successful responses
                    if (!response || response.status !== 200 || response.type !== 'basic') {
                        return response;
                    }

                    // Clone the response
                    const responseToCache = response.clone();
                    caches.open(CACHE_NAME).then(cache => {
                        cache.put(event.request, responseToCache);
                    });

                    return response;
                }).catch(() => {
                    // Return offline page or cached response
                    return caches.match('/certauto/index.html');
                });
            })
    );
});

// Push notifications (optional)
self.addEventListener('push', event => {
    const options = {
        body: event.data ? event.data.text() : 'CertAuto notification',
        icon: 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 192 192"><rect width="192" height="192" fill="%236366f1" rx="45"/><text y="140" font-size="120" fill="white" text-anchor="middle" x="96">🔒</text></svg>',
        badge: 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 192 192"><rect width="192" height="192" fill="%236366f1" rx="45"/><text y="140" font-size="120" fill="white" text-anchor="middle" x="96">🔒</text></svg>',
        vibrate: [200, 100, 200],
        tag: 'certauto-notification',
        requireInteraction: false
    };

    event.waitUntil(self.registration.showNotification('CertAuto', options));
});

// Background sync (optional)
self.addEventListener('sync', event => {
    if (event.tag === 'sync-certauto') {
        event.waitUntil(syncCertAutoData());
    }
});

async function syncCertAutoData() {
    try {
        // Implement sync logic here
        console.log('Syncing CertAuto data');
    } catch (error) {
        console.error('Sync failed:', error);
        throw error;
    }
}

// Message handling
self.addEventListener('message', event => {
    if (event.data && event.data.type === 'SKIP_WAITING') {
        self.skipWaiting();
    }

    if (event.data && event.data.type === 'CLEAR_CACHE') {
        caches.delete(CACHE_NAME);
    }
});

console.log('Service Worker registered for CertAuto');
