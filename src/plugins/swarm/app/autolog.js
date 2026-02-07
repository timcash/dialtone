import Autobase from 'autobase'
import Corestore from 'corestore'
import Hyperswarm from 'hyperswarm'
import b4a from 'b4a'
import crypto from 'hypercore-crypto'
import fs from 'bare-fs'
import path from 'bare-path'
import os from 'bare-os'

export class AutoLog {
  constructor(opts = {}) {
    const { topic, storage, bootstrap = null, requireBootstrap = false, logId = '' } = opts
    this.topicName = topic
    this.store = new Corestore(storage)
    this.base = null
    this.bootstrap = bootstrap
    this.requireBootstrap = requireBootstrap
    this.swarm = opts.swarm || null
    this.keySwarm = opts.keySwarm || null
    this.ownSwarm = !opts.swarm
    this.ownKeySwarm = !opts.keySwarm
    this.seenWriterKeys = new Set()
    this.topic = crypto.hash(b4a.from(topic, 'utf8'))
    this.bootstrapTimeoutMs = opts.bootstrapTimeoutMs || 15000
    this.useCache = opts.useCache === true
    this.logId = logId
    this.localKey = null
    this.keyInterval = null
  }

  get prefix() {
    return this.logId ? `[autolog:${this.logId}]` : '[autolog]'
  }

  async ready() {
    if (!this.bootstrap && this.useCache) {
      this.bootstrap = loadBootstrapCache(this.topicName)
    }

    if (!this.bootstrap) {
      this.bootstrap = loadLocalRegistry(this.topicName)
    }

    if (!this.bootstrap && this.requireBootstrap) {
      console.log(`${this.prefix} Discovering bootstrap key for "${this.topicName}"...`)
      this.bootstrap = await this.discoverBootstrapKey()
      if (!this.bootstrap) {
        console.log(`${this.prefix} Still no bootstrap after first try, waiting in loop...`)
        this.bootstrap = await this.waitForBootstrapKey()
      }
    }

    this.base = new Autobase(this.store, this.bootstrap, {
      valueEncoding: 'json',
      open: store => store.get('autolog', { valueEncoding: 'json' }),
      apply: applyLog
    })

    await this.base.ready()
    this.localKey = b4a.toString(this.base.local.key, 'hex').slice(0, 6)
    const key = b4a.toString(this.base.key, 'hex')
    console.log(`${this.prefix} [${this.localKey}] Base ready. key=0x${key.slice(0, 8)}... (topic: ${this.topicName})`)

    saveLocalRegistry(this.topicName, key)
    if (this.useCache) saveBootstrapCache(this.topicName, key)

    if (!this.swarm) {
      this.swarm = new Hyperswarm()
    }

    this.swarm.on('connection', (socket, info) => {
      const peerKey = b4a.toString(info.publicKey, 'hex').slice(0, 12)
      const isMain = info.topics.some(t => b4a.equals(t, this.topic))

      if (isMain) {
        console.log(`${this.prefix} [${this.localKey}] Main topic connection with ${peerKey}...`)
        const stream = this.base.replicate(info.client)
        socket.pipe(stream).pipe(socket)
      }
      socket.on('error', () => { })
    })

    this.swarm.join(this.topic, { server: true, client: true })
    await this.startKeyExchange()
    await this.base.update()
  }

  async startKeyExchange() {
    const keyTopicNamespace = this.topicName + ':bootstrap'
    const keyTopic = crypto.hash(b4a.from(keyTopicNamespace, 'utf8'))

    if (!this.keySwarm) {
      this.keySwarm = new Hyperswarm()
    }

    const broadcast = () => {
      const keyHex = b4a.toString(this.base.key, 'hex')
      const writerHex = b4a.toString(this.base.local.key, 'hex')
      const msg = `TOPIC:${this.topicName}\nBASE_KEY:${keyHex}\nWRITER_KEY:${writerHex}\n`

      for (const socket of this.keySwarm.connections) {
        socket.write(msg)
      }
    }

    this.keySwarm.on('connection', (socket, info) => {
      const peerKey = b4a.toString(info.publicKey, 'hex').slice(0, 12)

      // Only handle if this socket is relevant to our keyTopic
      const isRelevant = info.topics.some(t => b4a.equals(t, keyTopic))
      if (!isRelevant) return

      console.log(`${this.prefix} [${this.localKey}] KeySwarm connection with ${peerKey}...`)

      // Initial handshake
      const keyHex = b4a.toString(this.base.key, 'hex')
      const writerHex = b4a.toString(this.base.local.key, 'hex')
      socket.write(`TOPIC:${this.topicName}\nBASE_KEY:${keyHex}\nWRITER_KEY:${writerHex}\n`)

      let socketTopic = null
      const onData = (data) => {
        const lines = data.toString().split('\n')
        for (const line of lines) {
          if (line.startsWith('TOPIC:')) socketTopic = line.slice('TOPIC:'.length).trim()
          if (socketTopic !== this.topicName) continue

          if (line.startsWith('WRITER_KEY:')) {
            const hex = line.slice('WRITER_KEY:'.length).trim()
            if (hex && hex.length === 64) {
              this.addWriter(hex).catch(() => { })
            }
          }
        }
      }
      socket.on('data', onData)
      socket.on('error', () => { })
      socket.on('close', () => {
        socket.removeListener('data', onData)
      })
    })

    const discovery = this.keySwarm.join(keyTopic, { server: true, client: true })
    discovery.flushed().then(() => {
      console.log(`${this.prefix} [${this.localKey}] KeySwarm topic joined and flushed.`)
    })
    this.keyInterval = setInterval(broadcast, 5000)
    console.log(`${this.prefix} [${this.localKey}] Joined KeySwarm topic for ${this.topicName}`)
  }

  async stopKeyExchange() {
    if (this.keyInterval) clearInterval(this.keyInterval)
    if (this.ownKeySwarm && this.keySwarm) {
      await this.keySwarm.destroy()
      this.keySwarm = null
    }
  }

  async append(event) {
    if (typeof event !== 'object') event = { value: event }
    if (!this.base.writable) await this.waitWritable()
    await this.base.append({ type: 'session', event, timestamp: Date.now() })
    await this.base.update()
  }

  async waitWritable() {
    if (this.base.writable) return
    console.log(`${this.prefix} [${this.localKey}] Waiting for writer authorization...`)
    while (!this.base.writable) {
      await this.base.update()
      await new Promise(r => setTimeout(r, 1000))
    }
    console.log(`${this.prefix} [${this.localKey}] Node authorized. Writable now.`)
  }

  async list() {
    await this.base.update()
    const log = this.base.view
    if (!log) return []
    const entries = []
    for (let i = 0; i < log.length; i++) {
      const node = await log.get(i)
      if (node) entries.push(node)
    }
    return entries
  }

  get length() {
    return this.base.view ? this.base.view.length : 0
  }

  async close() {
    await this.stopKeyExchange()
    if (this.ownSwarm && this.swarm) await this.swarm.destroy()
    if (this.store) await this.store.close()
  }

  async waitForBootstrapKey() {
    while (!this.bootstrap) {
      this.bootstrap = await this.discoverBootstrapKey()
      if (this.bootstrap) break
      await new Promise((r) => setTimeout(r, 2000))
    }
    return this.bootstrap
  }

  async discoverBootstrapKey() {
    const keyTopicNamespace = this.topicName + ':bootstrap'
    const keyTopic = crypto.hash(b4a.from(keyTopicNamespace, 'utf8'))
    const swarm = new Hyperswarm()

    const core = this.store.get('autolog')
    await core.ready()
    const writerHex = b4a.toString(core.key, 'hex')

    return await new Promise((resolve) => {
      let resolved = false
      const done = (key) => {
        if (resolved) return
        resolved = true
        swarm.destroy().finally(() => resolve(key))
      }

      const timeout = setTimeout(() => done(null), this.bootstrapTimeoutMs)

      swarm.on('connection', (socket, info) => {
        const peerKey = b4a.toString(info.publicKey, 'hex').slice(0, 12)
        socket.write(`TOPIC:${this.topicName}\nWRITER_KEY:${writerHex}\n`)

        let socketTopic = null
        socket.on('data', (data) => {
          const lines = data.toString().split('\n')
          for (const line of lines) {
            if (line.startsWith('TOPIC:')) socketTopic = line.slice('TOPIC:'.length).trim()
            if (socketTopic !== this.topicName) continue

            if (line.startsWith('BASE_KEY:')) {
              const baseKey = line.slice('BASE_KEY:'.length).trim()
              console.log(`${this.prefix} [DISCOVER] Received BASE_KEY for ${this.topicName} from ${peerKey}: 0x${baseKey.slice(0, 8)}...`)
              clearTimeout(timeout)
              done(baseKey)
              break
            }
          }
        })
        socket.on('error', () => { })
      })

      const discovery = swarm.join(keyTopic, { server: true, client: true })
      discovery.flushed().then(() => {
        console.log(`${this.prefix} [DISCOVER] Joined KeySwarm topic for ${this.topicName}...`)
      })
      timeout.unref?.()
    })
  }

  async addWriter(writerKey) {
    const key = normalizeWriterKey(writerKey)
    const keyHex = b4a.toString(key, 'hex')
    if (this.seenWriterKeys.has(keyHex)) return
    this.seenWriterKeys.add(keyHex)
    console.log(`${this.prefix} [${this.localKey}] New writer authorized: ${keyHex.slice(0, 12)}...`)
    await this.base.append({ addWriter: keyHex })
    await this.base.update()
  }
}

async function applyLog(nodes, view, host) {
  for (const node of nodes) {
    const value = node.value
    if (value && value.addWriter) {
      await host.addWriter(b4a.from(value.addWriter, 'hex'), { indexer: true })
      continue
    }
    if (!value || value.type !== 'session' || !value.event) continue
    await view.append(value.event)
  }
}

function normalizeWriterKey(writerKey) {
  if (b4a.isBuffer(writerKey)) return writerKey
  if (typeof writerKey === 'string') return b4a.from(writerKey, 'hex')
  throw new Error('writerKey must be a hex string or buffer')
}

function loadBootstrapCache(topicName) {
  try {
    const filePath = path.join(os.homedir(), '.dialtone', 'swarm', 'autobase-cache.json')
    if (!fs.existsSync(filePath)) return null
    const cache = JSON.parse(fs.readFileSync(filePath, 'utf8'))
    return cache?.[topicName] || null
  } catch { return null }
}

function saveBootstrapCache(topicName, keyHex) {
  try {
    const filePath = path.join(os.homedir(), '.dialtone', 'swarm', 'autobase-cache.json')
    fs.mkdirSync(path.dirname(filePath), { recursive: true })
    const cache = fs.existsSync(filePath) ? JSON.parse(fs.readFileSync(filePath, 'utf8') || '{}') : {}
    cache[topicName] = keyHex
    fs.writeFileSync(filePath, JSON.stringify(cache, null, 2))
  } catch { }
}

function localRegistry() {
  if (!globalThis.__dialtoneAutobaseKeys) globalThis.__dialtoneAutobaseKeys = {}
  return globalThis.__dialtoneAutobaseKeys
}

function loadLocalRegistry(topicName) {
  return localRegistry()[topicName] || null
}

function saveLocalRegistry(topicName, keyHex) {
  localRegistry()[topicName] = keyHex
}
