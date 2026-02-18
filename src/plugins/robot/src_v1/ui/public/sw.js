// Self-destructing Service Worker to fix stale cache issues
self.addEventListener('install', (event) => {
  console.log('[SW] Installing cleanup worker...');
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  console.log('[SW] Activating cleanup worker...');
  event.waitUntil(
    self.registration.unregister().then(() => {
      console.log('[SW] Unregistered old worker.');
      return self.clients.matchAll();
    }).then((clients) => {
      // Force reload all clients to get fresh content
      clients.forEach((client) => client.navigate(client.url));
    })
  );
});
