import Autobase from 'autobase'
import Corestore from 'corestore'
import Hyperswarm from 'hyperswarm'
import b4a from 'b4a'
import crypto from 'hypercore-crypto'

export class AutoLog {
  constructor (opts = {}) {
    const { topic, storage, bootstrap = null, logId = '' } = opts
    this.topicName = topic
    this.store = new Corestore(storage)
    this.base = null
    this.bootstrap = bootstrap
    
    // MDNS is crucial for local testing stability
    this.swarm = opts.swarm || new Hyperswarm({ mdns: true })
    this.keySwarm = opts.keySwarm || new Hyperswarm({ mdns: true })
    this.ownSwarm = !opts.swarm
    this.ownKeySwarm = !opts.keySwarm
    
    this.seenWriterKeys = new Set()
    this.topic = crypto.hash(b4a.from(topic, 'utf8'))
    this.keyTopic = crypto.hash(b4a.from(topic + ':bootstrap', 'utf8'))
    
    this.logId = logId
    this.localKey = null
    this.keyInterval = null
    this.pulseInterval = opts.pulseInterval || 5000
  }

  get prefix () {
    return this.logId ? `[autolog_v2:${this.logId}]` : '[autolog_v2]'
  }

  async ready () {
    const start = Date.now()
    
    // Ensure local key is ready
    const core = this.store.get({ name: 'autolog' })
    await core.ready()
    const localKeyHex = b4a.toString(core.key, 'hex')

    if (!this.bootstrap) {
      this.bootstrap = localKeyHex
    }

    await this._initBase()

    // 1. Key Swarm (Handshake)
    this.keySwarm.on('connection', (socket, info) => {
      this._handleKeyHandshake(socket)
    })
    const discoveryKeys = this.keySwarm.join(this.keyTopic, { server: true, client: true })

    // 2. Data Swarm (Replication)
    this.swarm.on('connection', (socket, info) => {
      if (this.base) {
        this.store.replicate(socket)
      }
    })
    const discoveryData = this.swarm.join(this.topic, { server: true, client: true })
    
    await Promise.all([discoveryKeys.flushed(), discoveryData.flushed()])
    
    // 3. Periodic Pulse
    this.keyInterval = setInterval(() => {
      if (this.base) {
        this.base.update().catch(() => {})
        this._broadcastKeys()
      }
    }, this.pulseInterval)

    await this.base.update()
    console.log(`${this.prefix} [${this.localKey}] Ready (${Date.now() - start}ms)`)
  }

  _handleKeyHandshake (socket) {
    const sendPulse = () => {
      if (!this.base) return
      const writerHex = b4a.toString(this.base.local.key, 'hex')
      const baseKeyHex = b4a.toString(this.base.key, 'hex')
      socket.write(`TOPIC:${this.topicName}\nBASE_KEY:${baseKeyHex}\nWRITER_KEY:${writerHex}\n`)
    }

    sendPulse()

    let buffer = ''
    const onData = (data) => {
      buffer += data.toString()
      const lines = buffer.split('\n')
      buffer = lines.pop()

      for (const line of lines) {
        if (!line.includes(':')) continue
        const [cmd, val] = [line.slice(0, line.indexOf(':')), line.slice(line.indexOf(':') + 1).trim()]
        
        if (cmd === 'WRITER_KEY' && val.length === 64) {
          this.addWriter(val).catch(() => {})
        }
      }
    }
    socket.on('data', onData)
    socket.on('close', () => socket.removeListener('data', onData))
  }

  _broadcastKeys () {
    if (!this.base) return
    const writerHex = b4a.toString(this.base.local.key, 'hex')
    const baseKeyHex = b4a.toString(this.base.key, 'hex')
    const msg = `TOPIC:${this.topicName}\nBASE_KEY:${baseKeyHex}\nWRITER_KEY:${writerHex}\n`
    for (const socket of this.keySwarm.connections) {
      socket.write(msg)
    }
  }

  async _initBase () {
    if (this.base) return
    this.base = new Autobase(this.store, this.bootstrap, {
      valueEncoding: 'json',
      open: store => store.get('autolog', { valueEncoding: 'json' }),
      apply: applyLog
    })
    await this.base.ready()
    this.localKey = b4a.toString(this.base.local.key, 'hex').slice(0, 6)
  }

  async append (data) {
    if (!this.base?.writable) await this.waitWritable()
    await this.base.append({ type: 'log', data, timestamp: Date.now() })
    await this.base.update()
  }

  async tail (n = 10) {
    if (!this.base) return []
    await this.base.update()
    const log = this.base.view
    if (!log) return []
    const start = Math.max(0, log.length - n)
    const entries = []
    for (let i = start; i < log.length; i++) {
      entries.push(await log.get(i))
    }
    return entries
  }

  async getHash () {
    if (!this.base) return 'not-ready'
    await this.base.update()
    const log = this.base.view
    if (!log || log.length === 0) return 'empty'
    const last = await log.get(log.length - 1)
    return `${log.length}-${last?.timestamp || 0}`
  }

  async waitWritable () {
    while (!this.base) await new Promise(r => setTimeout(r, 500))
    if (this.base.writable) return
    console.log(`${this.prefix} [${this.localKey}] Waiting for writer auth...`)
    while (!this.base.writable) {
      await this.base.update() 
      if (this.base.writable) break
      await new Promise(r => setTimeout(r, 1000))
    }
    console.log(`${this.prefix} [${this.localKey}] Authorized.`)
  }

  async addWriter (writerKey) {
    if (!this.base?.writable) return
    const keyHex = b4a.toString(b4a.from(writerKey, 'hex'), 'hex')
    if (this.seenWriterKeys.has(keyHex)) return
    if (keyHex === b4a.toString(this.base.local.key, 'hex')) return
    
    this.seenWriterKeys.add(keyHex)
    console.log(`${this.prefix} [${this.localKey}] Authorizing peer: ${keyHex.slice(0, 12)}...`)
    await this.base.append({ addWriter: keyHex })
    await this.base.update()
  }

  async sync () {
    if (!this.base) return
    await this.base.update()
    if (this.base.writable) {
      await this.base.ack()
      await this.base.update()
    }
  }

  async close () {
    if (this.keyInterval) clearInterval(this.keyInterval)
    if (this.ownKeySwarm) await this.keySwarm.destroy()
    if (this.ownSwarm) await this.swarm.destroy()
    await this.store.close()
  }
}

async function applyLog (nodes, view, host) {
  for (const node of nodes) {
    const val = node.value
    if (val.addWriter) {
      await host.addWriter(b4a.from(val.addWriter, 'hex'), { indexer: true })
      continue
    }
    if (val.type === 'log') {
      await view.append(val)
    }
  }
}
