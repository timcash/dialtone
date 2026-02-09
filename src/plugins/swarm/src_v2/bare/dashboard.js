import fs from 'bare-fs'
import path from 'bare-path'
import http from 'bare-http1'

const appDir = Pear.config?.appDir || '.'
const uiDist = path.join(appDir, 'ui', 'dist')

const server = http.createServer((req, res) => {
  const url = req.url === '/' ? '/index.html' : req.url
  
  // API Endpoints
  if (url === '/api/status') {
    res.setHeader('Content-Type', 'application/json')
    res.end(JSON.stringify({ status: 'active', peers: 0 }))
    return
  }

  // Static File Serving (Vite Dist)
  const filePath = path.join(uiDist, url)
  if (fs.existsSync(filePath) && !fs.statSync(filePath).isDirectory()) {
    const ext = path.extname(filePath)
    const contentTypes = {
      '.html': 'text/html',
      '.js': 'text/javascript',
      '.css': 'text/css',
      '.png': 'image/png'
    }
    res.setHeader('Content-Type', contentTypes[ext] || 'text/plain')
    res.end(fs.readFileSync(filePath))
    return
  }

  // Fallback to index.html for SPA
  const indexHtml = path.join(uiDist, 'index.html')
  if (fs.existsSync(indexHtml)) {
    res.setHeader('Content-Type', 'text/html')
    res.end(fs.readFileSync(indexHtml))
    return
  }

  res.statusCode = 404
  res.end('Not Found')
})

server.listen(4000, '127.0.0.1', () => {
  console.log('[swarm] Dashboard server listening at http://127.0.0.1:4000')
})