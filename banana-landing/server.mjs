import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';

const ROOT = '/app/dist';
const API = process.env.API_ORIGIN || 'http://backend:8080';
const PORT = Number(process.env.PORT || 80);

const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.ico': 'image/x-icon',
  '.woff2': 'font/woff2',
  '.map': 'application/json',
};

function sendFile(res, file) {
  const ext = path.extname(file);
  res.writeHead(200, {
    'Content-Type': MIME[ext] || 'application/octet-stream',
    'Cache-Control': file.includes('/assets/') ? 'public, max-age=31536000, immutable' : 'no-cache',
  });
  fs.createReadStream(file).pipe(res);
}

http.createServer((req, res) => {
  const url = new URL(req.url, 'http://x');

  // API и healthz — прозрачно на Go, включая загрузку файлов потоком
  if (url.pathname.startsWith('/api/') || url.pathname === '/healthz') {
    const target = new URL(API + url.pathname + url.search);
    const headers = { ...req.headers, host: target.host };
    const up = http.request(target, { method: req.method, headers }, (r) => {
      res.writeHead(r.statusCode, r.headers);
      r.pipe(res);
    });
    up.on('error', () => {
      res.writeHead(502, { 'Content-Type': 'application/json' });
      res.end('{"error":{"message":"backend unavailable"}}');
    });
    req.pipe(up);
    return;
  }

  // статика; чего нет — index.html (SPA)
  const rel = decodeURIComponent(url.pathname).replace(/^\/+/, '');
  const file = path.join(ROOT, rel);
  if (rel && file.startsWith(ROOT) && fs.existsSync(file) && fs.statSync(file).isFile()) {
    return sendFile(res, file);
  }
  sendFile(res, path.join(ROOT, 'index.html'));
}).listen(PORT, () => console.log('banana-landing on :' + PORT + ' → ' + API));