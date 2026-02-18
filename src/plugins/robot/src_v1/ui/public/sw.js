const CACHE_NAME = 'dialtone-robot-v1';
const PRECACHE = [
  '/',
  '/index.html',
  '/manifest.json',
  '/icon-192.png',
  '/icon-512.png'
];

self.addEventListener('install', (e) => {
  self.skipWaiting(); // Activate immediately
  e.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(PRECACHE))
  );
});

self.addEventListener('activate', (e) => {
  e.waitUntil(self.clients.claim()); // Take control immediately
});

self.addEventListener('fetch', (e) => {
  // Skip non-GET or cross-origin requests if needed, but for now cache everything same-origin
  if (e.request.method !== 'GET') return;

  e.respondWith(
    caches.match(e.request).then((cached) => {
      if (cached) return cached;

      return fetch(e.request).then((response) => {
        // Cache valid responses dynamically (JS, CSS, etc.)
        if (!response || response.status !== 200 || response.type !== 'basic') {
          return response;
        }
        const responseToCache = response.clone();
        caches.open(CACHE_NAME).then((cache) => {
          cache.put(e.request, responseToCache);
        });
        return response;
      }).catch(() => {
        // Offline fallback for navigation
        if (e.request.mode === 'navigate') {
            return caches.match('/index.html');
        }
      });
    })
  );
});