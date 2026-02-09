import path from 'bare-path'
import fs from 'bare-fs'
import http from 'bare-http1'

const appDir = Pear.config?.appDir || '.'

const server = http.createServer((req, res) => {
  if (req.url === '/' || req.url === '/index.html') {
    res.setHeader('Content-Type', 'text/html')
    res.end(fs.readFileSync(path.join(appDir, 'dashboard.html')))
  } else if (req.url === '/status') {
    res.setHeader('Content-Type', 'application/json')
    res.end(JSON.stringify({ status: 'active', time: Date.now() }))
  } else {
    res.statusCode = 404
    res.end()
  }
})

server.listen(4000, '127.0.0.1', () => {
  console.log('[swarm] Dashboard server listening at http://127.0.0.1:4000')
})
