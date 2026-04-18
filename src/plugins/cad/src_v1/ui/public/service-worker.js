const CACHE_NAME = 'dialtone-cad-shell-v1';
const OFFLINE_URL = new URL('./offline.html', self.location.href).toString();
const SHELL_URLS = [
  new URL('./', self.location.href).toString(),
  new URL('./index.html', self.location.href).toString(),
  new URL('./manifest.webmanifest', self.location.href).toString(),
  OFFLINE_URL,
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(SHELL_URLS)).then(() => self.skipWaiting()),
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(
        keys.map((key) => {
          if (key === CACHE_NAME) {
            return Promise.resolve();
          }
          return caches.delete(key);
        }),
      ),
    ).then(() => self.clients.claim()),
  );
});

self.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET') {
    return;
  }

  const requestURL = new URL(event.request.url);
  if (requestURL.origin !== self.location.origin) {
    return;
  }

  if (event.request.mode === 'navigate') {
    event.respondWith(
      fetch(event.request).catch(async () => {
        const cachedIndex = await caches.match(new URL('./index.html', self.location.href).toString());
        return cachedIndex || caches.match(OFFLINE_URL);
      }),
    );
    return;
  }

  event.respondWith(
    caches.match(event.request).then((cached) => {
      if (cached) {
        return cached;
      }
      return fetch(event.request).then((response) => {
        if (!response || response.status !== 200 || response.type !== 'basic') {
          return response;
        }
        const copy = response.clone();
        event.waitUntil(caches.open(CACHE_NAME).then((cache) => cache.put(event.request, copy)));
        return response;
      });
    }),
  );
});
