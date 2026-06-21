const CACHE_NAME = 'anban-childweb-v11';
const SHELL_REVISION = 'mind-view-20260620-2';
const APP_SHELL = [
  './',
  './index.html',
  './app.js',
  './config.js',
  './integration-core.js',
  './not-implemented.js',
  './api/client.js',
  './dist.css',
  './fonts/fonts.css',
  './fonts/icons.css',
  './manifest.webmanifest',
  './icons/icon-192.png',
  './icons/icon-512.png',
];

self.addEventListener('install', (event) => {
  void SHELL_REVISION;
  event.waitUntil(caches.open(CACHE_NAME).then((cache) => cache.addAll(APP_SHELL)));
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) => Promise.all(
      keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key)),
    )),
  );
  self.clients.claim();
});

self.addEventListener('fetch', (event) => {
  const { request } = event;
  const url = new URL(request.url);
  if (url.pathname.startsWith('/api/')) {
    event.respondWith(fetch(request));
    return;
  }
  if (request.method !== 'GET') return;
  event.respondWith(caches.match(request).then((cached) => cached || fetch(request)));
});
