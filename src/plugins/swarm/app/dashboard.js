import fs from 'bare-fs'
import path from 'bare-path'
import os from 'bare-os'
import http from 'bare-http1'

const server = http.createServer((req, res) => {
    if (req.url === '/' || req.url === '/index.html') {
        res.setHeader('Content-Type', 'text/html')
        res.end(fs.readFileSync(path.join(Pear.config.appDir, 'dashboard.html')))
    } else {
        res.statusCode = 404
        res.end()
    }
})

server.listen(4000, '127.0.0.1', () => {
    console.log('[swarm] Dashboard server listening at http://127.0.0.1:4000')
})
