import fs from 'bare-fs'
import path from 'bare-path'
import os from 'bare-os'
import http from 'bare-http1'
import { spawn } from 'bare-subprocess'

const appDir = Pear.config?.appDir || '.'

function findDialtoneScript(startDir) {
    let dir = startDir
    for (let i = 0; i < 8; i += 1) {
        const candidate = path.join(dir, 'dialtone.sh')
        if (fs.existsSync(candidate)) {
            return candidate
        }
        const parent = path.dirname(dir)
        if (parent === dir) break
        dir = parent
    }
    return null
}

const repoRoot = globalThis.DIALTONE_REPO || Bare?.env?.DIALTONE_REPO || null
const dialtoneScript = repoRoot ? path.join(repoRoot, 'dialtone.sh') : findDialtoneScript(appDir)
const resolvedRepoRoot = repoRoot || (dialtoneScript ? path.dirname(dialtoneScript) : appDir)

const swarmDir = path.join(os.homedir(), '.dialtone', 'swarm')

function readJsonBody(req) {
    return new Promise((resolve, reject) => {
        let data = ''
        req.on('data', (chunk) => {
            data += chunk
        })
        req.on('end', () => {
            if (!data) return resolve({})
            try {
                resolve(JSON.parse(data))
            } catch (err) {
                reject(err)
            }
        })
        req.on('error', reject)
    })
}

function runDialtone(args) {
    return new Promise((resolve, reject) => {
        if (!dialtoneScript) {
            reject(new Error('dialtone.sh not found. Run dashboard from repo root.'))
            return
        }
        const proc = spawn('bash', [dialtoneScript, ...args], { cwd: resolvedRepoRoot })
        let stdout = ''
        let stderr = ''
        proc.stdout?.on('data', (chunk) => { stdout += chunk })
        proc.stderr?.on('data', (chunk) => { stderr += chunk })
        proc.on('close', (code) => {
            if (code === 0) {
                resolve({ stdout, stderr })
            } else {
                reject(new Error(stderr || `Command failed with code ${code}`))
            }
        })
        proc.on('error', reject)
    })
}

function readStatus() {
    try {
        const files = fs.readdirSync(swarmDir)
        const statusFiles = files.filter((f) => f.startsWith('status_'))
        const nodes = statusFiles.map((file) => {
            try {
                const data = fs.readFileSync(path.join(swarmDir, file))
                return JSON.parse(data)
            } catch (err) {
                return { error: err?.message || String(err), file }
            }
        })
        return { nodes }
    } catch (err) {
        return { error: err?.message || String(err), nodes: [] }
    }
}

const server = http.createServer((req, res) => {
    try {
        const url = req.url || '/'
        if (url === '/' || url === '/index.html') {
            res.setHeader('Content-Type', 'text/html')
            const html = fs.readFileSync(path.join(appDir, 'dashboard.html'))
            res.end(html)
            return
        }
        if (url === '/status') {
            res.setHeader('Content-Type', 'application/json')
            res.end(JSON.stringify(readStatus()))
            return
        }
        if (url === '/start' && req.method === 'POST') {
            readJsonBody(req).then(async (body) => {
                const topic = String(body.topic || '').trim()
                const name = String(body.name || '').trim()
                if (!topic) {
                    res.statusCode = 400
                    res.end(JSON.stringify({ error: 'Topic is required' }))
                    return
                }
                const args = ['swarm', 'start', topic]
                if (name) args.push(name)
                try {
                    await runDialtone(args)
                    res.setHeader('Content-Type', 'application/json')
                    res.end(JSON.stringify({ ok: true }))
                } catch (err) {
                    res.statusCode = 500
                    res.end(JSON.stringify({ error: err?.message || String(err) }))
                }
            }).catch((err) => {
                res.statusCode = 400
                res.end(JSON.stringify({ error: err?.message || String(err) }))
            })
            return
        }
        if (url === '/stop' && req.method === 'POST') {
            readJsonBody(req).then(async (body) => {
                const pid = String(body.pid || '').trim()
                if (!pid) {
                    res.statusCode = 400
                    res.end(JSON.stringify({ error: 'PID is required' }))
                    return
                }
                try {
                    await runDialtone(['swarm', 'stop', pid])
                    res.setHeader('Content-Type', 'application/json')
                    res.end(JSON.stringify({ ok: true }))
                } catch (err) {
                    res.statusCode = 500
                    res.end(JSON.stringify({ error: err?.message || String(err) }))
                }
            }).catch((err) => {
                res.statusCode = 400
                res.end(JSON.stringify({ error: err?.message || String(err) }))
            })
            return
        }
        res.statusCode = 404
        res.end()
    } catch (err) {
        console.error('[swarm] Dashboard request failed:', err?.message || err)
        res.statusCode = 500
        res.end('Dashboard error')
    }
})

server.listen(4000, '127.0.0.1', () => {
    console.log('[swarm] Dashboard server listening at http://127.0.0.1:4000')
})
