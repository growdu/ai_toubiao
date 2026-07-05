// Pure-Node static + reverse proxy: serves the built web SPA from ./dist
// and forwards /api/* to the live api-gateway. Zero external deps so
// it works regardless of web/node_modules state.
//
// Usage:  node serve.cjs [PORT]  (PORT defaults to 3000)
//         GATEWAY=http://host:port node serve.cjs
const http = require('http');
const fs = require('fs');
const path = require('path');

const PORT = Number(process.argv[2] || process.env.PORT || 3000);
const GATEWAY = process.env.GATEWAY || 'http://localhost:7080';
const DIST = path.resolve(__dirname, 'dist');

const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.js':   'application/javascript; charset=utf-8',
  '.mjs':  'application/javascript; charset=utf-8',
  '.css':  'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg':  'image/svg+xml',
  '.png':  'image/png',
  '.jpg':  'image/jpeg',
  '.ico':  'image/x-icon',
  '.woff2':'font/woff2',
  '.map':  'application/json; charset=utf-8',
};

function gatewayRequest(req, res) {
  const target = new URL(GATEWAY);
  const opts = {
    hostname: target.hostname,
    port: target.port || 80,
    method: req.method,
    path: req.url,
    headers: { ...req.headers, host: target.host },
  };
  const proxy = http.request(opts, (upstream) => {
    res.writeHead(upstream.statusCode, upstream.headers);
    upstream.pipe(res);
  });
  proxy.on('error', (err) => {
    res.writeHead(502, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ error: 'gateway_proxy', detail: String(err) }));
  });
  // Inject a default tenant header for unauthenticated calls.
  if (!req.headers['x-tenant-id']) {
    proxy.setHeader('x-tenant-id', '11111111-1111-1111-1111-111111111111');
  }
  req.pipe(proxy);
}

function serveStatic(req, res) {
  let urlPath = decodeURIComponent(req.url.split('?')[0]);
  if (urlPath === '/' || !path.extname(urlPath)) {
    // SPA fallback: send index.html for client-side routes.
    urlPath = '/index.html';
  }
  const full = path.normalize(path.join(DIST, urlPath));
  if (!full.startsWith(DIST)) {
    res.writeHead(403);
    return res.end('forbidden');
  }
  fs.stat(full, (err, stat) => {
    if (err || !stat.isFile()) {
      res.writeHead(404);
      return res.end('not found');
    }
    const mime = MIME[path.extname(full).toLowerCase()] || 'application/octet-stream';
    res.writeHead(200, {
      'content-type': mime,
      'content-length': stat.size,
    });
    fs.createReadStream(full).pipe(res);
  });
}

const server = http.createServer((req, res) => {
  if (req.url && req.url.startsWith('/api/')) {
    return gatewayRequest(req, res);
  }
  if (req.url && req.url.startsWith('/api')) {
    return gatewayRequest(req, res);
  }
  serveStatic(req, res);
});

server.listen(PORT, '0.0.0.0', () => {
  console.log(`web: serving ${DIST} on :${PORT}, proxy /api -> ${GATEWAY}`);
});
