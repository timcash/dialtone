import fs from 'bare-fs'
import path from 'bare-path'
import http from 'bare-http1'
import { WebSocketServer } from 'bare-ws'
import { spawn } from 'bare-subprocess'
import b4a from 'b4a'

export function startDashboard(log, kv) {
  const appDir = Pear.config?.appDir || '.'
  const uiDist = path.join(appDir, 'ui', 'dist')

  const server = http.createServer((req, res) => {
    const url = req.url === '/' ? '/index.html' : req.url
    
    // 1. API Endpoints
    if (url === '/api/status') {
      res.setHeader('Content-Type', 'application/json')
      res.end(JSON.stringify({ status: 'active', topic: log.topicName }))
      return
    }

    if (url === '/api/data') {
      res.setHeader('Content-Type', 'application/json')
      Promise.all([kv.list(), log.tail(50)]).then(([kvData, logData]) => {
        res.end(JSON.stringify({ kv: kvData, log: logData }))
      }).catch(err => {
        res.statusCode = 500
        res.end(JSON.stringify({ error: err.message }))
      })
      return
    }

    if (url === '/api/kv/put' && req.method === 'POST') {
      readJsonBody(req).then(async (body) => {
        await kv.put(body.key, body.value)
        res.end(JSON.stringify({ ok: true }))
      })
      return
    }

    if (url === '/api/kv/del' && req.method === 'POST') {
      readJsonBody(req).then(async (body) => {
        // Hyperbee del implementation if needed, for now we just put null or similar
        // Actually AutoKV apply handles del if we pass type: 'del'
        await kv.base.append({ type: 'del', key: body.key })
        res.end(JSON.stringify({ ok: true }))
      })
      return
    }

    if (url === '/api/log/append' && req.method === 'POST') {
      readJsonBody(req).then(async (body) => {
        await log.append({ data: body.msg, timestamp: Date.now() })
        res.end(JSON.stringify({ ok: true }))
      })
      return
    }

    // 2. Static File Serving
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

    // Fallback
    const indexHtml = path.join(uiDist, 'index.html')
    if (fs.existsSync(indexHtml)) {
      res.setHeader('Content-Type', 'text/html')
      res.end(fs.readFileSync(indexHtml))
      return
    }

    res.statusCode = 404
    res.end('Not Found')
  })

  // 3. WebSocket Terminal
  const wss = new WebSocketServer({ server })
  wss.on('connection', (ws) => {
    console.log('[swarm] Terminal connected')
    let shell = null
    let inputBuffer = ''

    ws.on('message', (data) => {
      const msg = data.toString()
      
      if (msg === '\r' || msg === '\n') {
        ws.send('\r\n')
        if (inputBuffer.trim()) {
          const args = inputBuffer.trim().split(' ')
          const cmd = args.shift()
          
          // Simple command handling
          const dialtoneSh = path.join(Bare.cwd(), 'dialtone.sh')
          
          ws.send(`Running: ${cmd} ${args.join(' ')}\r\n`)
          
          shell = spawn('bash', [dialtoneSh, ...args])
          
          shell.stdout.on('data', (d) => ws.send(d))
          shell.stderr.on('data', (d) => ws.send(d))
          shell.on('close', (code) => {
            ws.send(`\r\nCommand finished with code ${code}\r\n$ `)
            shell = null
          })
        } else {
          ws.send('$ ')
        }
        inputBuffer = ''
      } else if (msg === '\u007f') { // Backspace
        if (inputBuffer.length > 0) {
          inputBuffer = inputBuffer.slice(0, -1)
          ws.send('\b \b')
        }
      } else {
        inputBuffer += msg
        ws.send(msg)
      }
    })
  })

  server.listen(4000, '127.0.0.1', () => {
    console.log('[swarm] Dashboard server listening at http://127.0.0.1:4000')
  })
}

function readJsonBody(req) {
  return new Promise((resolve) => {
    let data = ''
    req.on('data', (chunk) => { data += chunk })
    req.on('end', () => {
      try { resolve(JSON.parse(data)) } catch { resolve({}) }
    })
  })
}