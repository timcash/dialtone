import Autobase from 'autobase'
import Corestore from 'corestore'
import Hyperswarm from 'hyperswarm'
import Hyperbee from 'hyperbee'
import b4a from 'b4a'
import crypto from 'hypercore-crypto'

export class AutoKV {
  constructor (opts = {}) {
    const { topic, storage, bootstrap = null, logId = '' } = opts
    this.topicName = topic
    this.store = new Corestore(storage)
    this.base = null
    this.bee = null
    this.bootstrap = bootstrap
    
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
    return this.logId ? `[autokv_v2:${this.logId}]` : '[autokv_v2]'
  }

  async ready () {
    const start = Date.now()
    
    const core = this.store.get({ name: 'autokv' })
    await core.ready()
    const localKeyHex = b4a.toString(core.key, 'hex')

    if (!this.bootstrap) {
      this.bootstrap = localKeyHex
    }

    await this._initBase()

    this.keySwarm.on('connection', (socket, info) => {
      this._handleKeyHandshake(socket)
    })
    const discoveryKeys = this.keySwarm.join(this.keyTopic, { server: true, client: true })

    this.swarm.on('connection', (socket, info) => {
      if (this.base) {
        this.store.replicate(socket)
      }
    })
    const discoveryData = this.swarm.join(this.topic, { server: true, client: true })

    await Promise.all([discoveryKeys.flushed(), discoveryData.flushed()])

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
      open: store => store.get('autokv', { valueEncoding: 'json' }),
      apply: applyKv
    })
    await this.base.ready()
    this.localKey = b4a.toString(this.base.local.key, 'hex').slice(0, 6)
    this.bee = new Hyperbee(this.base.view, { extension: false, keyEncoding: 'utf-8', valueEncoding: 'json' })
    await this.bee.ready()
  }

  async put (key, value) {
    if (!this.base?.writable) await this.waitWritable()
    await this.base.append({ type: 'put', key, value })
    await this.base.update()
  }

  async get (key) {
    if (!this.base) return null
    await this.base.update()
    const entry = await this.bee.get(key)
    return entry?.value || null
  }

  async getHash () {
    if (!this.base) return 'not-ready'
    await this.base.update()
    return `${this.base.view.length}-${this.bee.version}`
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

async function applyKv (nodes, view, host) {
  const bee = new Hyperbee(view, { extension: false, keyEncoding: 'utf-8', valueEncoding: 'json' })
  await bee.ready()
  for (const node of nodes) {
    const val = node.value
    if (val.addWriter) {
      await host.addWriter(b4a.from(val.addWriter, 'hex'), { indexer: true })
      continue
    }
    if (val.type === 'put') await bee.put(val.key, val.value)
    else if (val.type === 'del') await bee.del(val.key)
  }
}
