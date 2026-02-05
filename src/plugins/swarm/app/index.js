import Hyperswarm from 'hyperswarm'
import b4a from 'b4a'
import crypto from 'hypercore-crypto'
import fs from 'bare-fs'
import path from 'bare-path'
import os from 'bare-os'

const mode = Pear.config.args[0]
if (mode === 'dashboard') {
  console.log('[swarm] Starting dashboard mode...')
  globalThis.DIALTONE_REPO = Pear.config.args[1] || ''
  try {
    await import('./dashboard.js')
  } catch (err) {
    console.error('[swarm] Dashboard failed to start:', err?.message || err)
    Bare.exit(1)
  }
} else {
  const swarm = new Hyperswarm()
  const topicName = mode || 'dialtone-default'
  const nameArg = Pear.config.args[1]
  const topic = crypto.hash(b4a.from(topicName, 'utf8'))

  const nodeName = nameArg || `${os.hostname?.() || 'node'}-${Bare.pid}`
  const statusPath = path.join(os.homedir(), '.dialtone', 'swarm', `status_${Bare.pid}.json`)
  const peerLatencies = new Map()

  console.log(`[swarm] Joining topic: ${topicName} (PID: ${Bare.pid})`)
  console.log(`[swarm] Topic hash: ${b4a.toString(topic, 'hex')}`)

  function updateStatus() {
    const status = {
      pid: Bare.pid,
      name: nodeName,
      topic: topicName,
      peers: swarm.connections.size,
      updated: new Date().toISOString(),
      latencies: Object.fromEntries(peerLatencies)
    }
    try {
      fs.writeFileSync(statusPath, JSON.stringify(status, null, 2))
    } catch (err) {
      console.error('[swarm] Failed to write status:', err.message)
    }
  }

  swarm.on('connection', (socket, info) => {
    const peerKey = b4a.toString(info.publicKey, 'hex')
    console.log(`[swarm] Connected to peer: ${peerKey}`)

    socket.on('data', (data) => {
      const message = b4a.toString(data)
      if (message.startsWith('ping:')) {
        socket.write(b4a.from('pong:' + message.split(':')[1]))
      } else if (message.startsWith('pong:')) {
        const sentTime = parseInt(message.split(':')[1])
        const latency = Date.now() - sentTime
        peerLatencies.set(peerKey.substring(0, 8), latency)
        updateStatus()
      } else {
        console.log(`[swarm] Data from ${peerKey}:`, message)
      }
    })

  // Start heartbeat
    const interval = setInterval(() => {
      socket.write(b4a.from('ping:' + Date.now()))
    }, 5000)

    socket.on('close', () => {
      clearInterval(interval)
      peerLatencies.delete(peerKey.substring(0, 8))
      updateStatus()
    })
  })

  const discovery = swarm.join(topic, { server: true, client: true })
  discovery.flushed().then(() => {
    console.log(`[swarm] Swarm joined and flushed for topic: ${topicName}`)
    updateStatus()
  })

// Keep alive and periodic update
  setInterval(updateStatus, 10000)

  Pear.teardown(async () => {
    console.log('[swarm] Shutting down...')
    try { fs.unlinkSync(statusPath) } catch (err) { }
    await swarm.destroy()
  })
}

