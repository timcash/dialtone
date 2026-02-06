import Autobase from 'autobase'
import Corestore from 'corestore'
import Hyperswarm from 'hyperswarm'
import Hyperbee from 'hyperbee'
import b4a from 'b4a'
import crypto from 'hypercore-crypto'
import fs from 'bare-fs'
import path from 'bare-path'
import os from 'bare-os'

export class AutoKV {
  constructor ({ topic, storage, bootstrap = null, keyPair = null, bootstrapTimeoutMs = 8000, keepBootstrapHost = true, useCache = true, requireBootstrap = false }) {
    this.topicName = topic
    this.store = new Corestore(storage)
    this.base = null
    this.bootstrap = bootstrap
    this.keyPair = keyPair
    this.bootstrapTimeoutMs = bootstrapTimeoutMs
    this.keepBootstrapHost = keepBootstrapHost
    this.useCache = useCache
    this.requireBootstrap = requireBootstrap
    this.swarm = null
    this.keySwarm = null
    this.seenWriterKeys = new Set()
    this.topic = crypto.hash(b4a.from(topic, 'utf8'))
    this.bee = null
  }

  async ready () {
    const env = getEnv()
    if (env.DIALTONE_SWARM_DISABLE_BOOTSTRAP_HOST === '1') {
      this.keepBootstrapHost = false
    }
    if (env.DIALTONE_SWARM_DISABLE_CACHE === '1') {
      this.useCache = false
    }
    const timeoutEnv = env.DIALTONE_SWARM_BOOTSTRAP_TIMEOUT_MS
    if (timeoutEnv && !Number.isNaN(Number(timeoutEnv))) {
      this.bootstrapTimeoutMs = Number(timeoutEnv)
    }

    if (!this.bootstrap) {
      if (this.useCache) {
        this.bootstrap = loadBootstrapCache(this.topicName)
      }
    }
    if (!this.bootstrap) {
      this.bootstrap = loadLocalRegistry(this.topicName)
      if (this.bootstrap) {
        console.log(`[autokv] Using local registry bootstrap for "${this.topicName}"`)
      }
    }
    if (!this.bootstrap) {
      console.log(`[autokv] Discovering bootstrap for topic "${this.topicName}"...`)
      this.bootstrap = await this.discoverBootstrapKey()
    }
    if (!this.bootstrap && this.requireBootstrap) {
      console.log('[autokv] Waiting for bootstrap key from swarm...')
      this.bootstrap = await this.waitForBootstrapKey()
    }

    if (this.bootstrap) {
      console.log(`[autokv] Using bootstrap key: ${this.bootstrap}`)
    } else {
      console.log('[autokv] No bootstrap found, creating new base')
    }

    this.base = new Autobase(this.store, this.bootstrap, {
      valueEncoding: 'json',
      open: openKv,
      apply: applyKv
    })

    await this.base.ready()
    console.log(`[autokv] Base ready. key=${b4a.toString(this.base.key, 'hex')}`)
    saveLocalRegistry(this.topicName, b4a.toString(this.base.key, 'hex'))
    if (this.useCache) {
      saveBootstrapCache(this.topicName, b4a.toString(this.base.key, 'hex'))
    }
    const view = this.base.view || this.base._viewStore.get('autokv', { valueEncoding: 'json' })
    this.bee = new Hyperbee(view, {
      extension: false,
      keyEncoding: 'utf-8',
      valueEncoding: 'json'
    })
    await this.bee.ready()

    this.swarm = new Hyperswarm(this.keyPair ? { keyPair: this.keyPair } : undefined)
    this.swarm.on('connection', (socket, info) => {
      const peerKey = b4a.toString(info.publicKey, 'hex')
      console.log(`[autokv] Swarm connection: ${peerKey}`)
      socket.on('error', () => {})
      const stream = this.base.replicate(info.client)
      socket.pipe(stream).pipe(socket)
    })
    const discovery = this.swarm.join(this.topic, { server: true, client: true })
    console.log(`[autokv] Joined topic: ${this.topicName}`)
    if (discovery?.flushed) {
      await flushWithTimeout(discovery, 8000, '[autokv] discovery.flushed')
    }
    if (this.keepBootstrapHost) {
      await this.startKeyExchange()
    }
    await flushWithTimeout(this.swarm, 8000, '[autokv] swarm.flush')
  }

  async waitForBootstrapKey () {
    while (!this.bootstrap) {
      this.bootstrap = await this.discoverBootstrapKey()
      if (this.bootstrap) break
      await new Promise((r) => setTimeout(r, 500))
    }
    return this.bootstrap
  }

  async addWriter (writerKey) {
    const key = normalizeWriterKey(writerKey)
    const keyHex = b4a.toString(key, 'hex')
    if (this.seenWriterKeys.has(keyHex)) return
    this.seenWriterKeys.add(keyHex)
    console.log(`[autokv] Adding writer: ${keyHex}`)
    await this.base.append({ addWriter: b4a.toString(key, 'hex') })
    await this.base.update()
  }

  async put (key, value) {
    await this.base.append({ type: 'kv', key, value })
    await this.base.update()
  }

  async get (key) {
    return this.bee.get(key)
  }

  async close () {
    await this.stopKeyExchange()
    await this.swarm.destroy()
    await this.store.close()
  }

  async discoverBootstrapKey () {
    const keyTopic = crypto.hash(b4a.from(this.topicName + ':bootstrap', 'utf8'))
    const swarm = new Hyperswarm(this.keyPair ? { keyPair: this.keyPair } : undefined)
    return await new Promise((resolve) => {
      let resolved = false
      const done = (key) => {
        if (resolved) return
        resolved = true
        swarm.destroy().finally(() => resolve(key))
      }

      const timeout = setTimeout(() => {
        console.log('[autokv] Bootstrap discovery timed out')
        done(null)
      }, this.bootstrapTimeoutMs)

      swarm.on('connection', (socket) => {
        console.log('[autokv] Bootstrap connection established')
        socket.on('error', () => {})
        socket.on('data', (data) => {
          const lines = data.toString().split('\n')
          for (const line of lines) {
            if (line.startsWith('BASE_KEY:')) {
              clearTimeout(timeout)
              console.log('[autokv] Bootstrap key received')
              done(line.slice('BASE_KEY:'.length).trim())
              break
            }
          }
        })
      })

      const discovery = swarm.join(keyTopic, { server: true, client: true })
      console.log(`[autokv] Joined bootstrap discovery topic: ${this.topicName}:bootstrap`)
      if (discovery?.flushed) {
        flushWithTimeout(discovery, this.bootstrapTimeoutMs, '[autokv] discovery.flushed').catch(() => {})
      }
      swarm.flush().catch(() => done(null))
      timeout.unref?.()
    })
  }

  async startKeyExchange () {
    const keyTopic = crypto.hash(b4a.from(this.topicName + ':bootstrap', 'utf8'))
    this.keySwarm = new Hyperswarm(this.keyPair ? { keyPair: this.keyPair } : undefined)
    this.keySwarm.on('connection', (socket) => {
      const keyHex = b4a.toString(this.base.key, 'hex')
      const writerHex = b4a.toString(this.base.local.key, 'hex')
      socket.on('error', () => {})
      socket.write(`BASE_KEY:${keyHex}\nWRITER_KEY:${writerHex}\n`)
      console.log(`[autokv] Sent bootstrap key + writer to peer`)
      socket.on('data', (data) => {
        const lines = data.toString().split('\n')
        for (const line of lines) {
          if (line.startsWith('WRITER_KEY:')) {
            const hex = line.slice('WRITER_KEY:'.length).trim()
            if (hex) this.addWriter(hex).catch(() => {})
          }
        }
      })
    })
    const discovery = this.keySwarm.join(keyTopic, { server: true, client: true })
    console.log(`[autokv] Joined bootstrap channel: ${this.topicName}:bootstrap`)
    if (discovery?.flushed) {
      await flushWithTimeout(discovery, 8000, '[autokv] keyDiscovery.flushed')
    }
    await flushWithTimeout(this.keySwarm, 8000, '[autokv] keySwarm.flush')
  }

  async stopKeyExchange () {
    if (!this.keySwarm) return
    const swarm = this.keySwarm
    this.keySwarm = null
    await swarm.destroy()
  }
}

function openKv (store) {
  return store.get('autokv', { valueEncoding: 'json' })
}

async function applyKv (nodes, view, host) {
  const bee = new Hyperbee(view, {
    extension: false,
    keyEncoding: 'utf-8',
    valueEncoding: 'json'
  })
  await bee.ready()
  for (const node of nodes) {
    const value = node.value
    if (value && value.addWriter) {
      const key = b4a.from(value.addWriter, 'hex')
      await host.addWriter(key, { indexer: true })
      continue
    }
    if (!value || value.type !== 'kv' || typeof value.key !== 'string') continue
    await bee.put(value.key, value.value)
  }
}

function normalizeWriterKey (writerKey) {
  if (b4a.isBuffer(writerKey)) return writerKey
  if (typeof writerKey === 'string') return b4a.from(writerKey, 'hex')
  throw new Error('writerKey must be a hex string or buffer')
}

function getEnv () {
  return globalThis.Pear?.config?.env || globalThis.process?.env || {}
}

function cachePath () {
  return path.join(os.homedir(), '.dialtone', 'swarm', 'autobase-cache.json')
}

function loadBootstrapCache (topicName) {
  try {
    const filePath = cachePath()
    if (!fs.existsSync(filePath)) return null
    const data = fs.readFileSync(filePath, 'utf8')
    const cache = JSON.parse(data)
    const entry = cache?.[topicName]
    return typeof entry === 'string' ? entry : null
  } catch {
    return null
  }
}

function saveBootstrapCache (topicName, keyHex) {
  try {
    const filePath = cachePath()
    fs.mkdirSync(path.dirname(filePath), { recursive: true })
    let cache = {}
    if (fs.existsSync(filePath)) {
      cache = JSON.parse(fs.readFileSync(filePath, 'utf8') || '{}')
    }
    cache[topicName] = keyHex
    fs.writeFileSync(filePath, JSON.stringify(cache, null, 2))
  } catch (err) {
    console.error('[autokv] Failed to write bootstrap cache:', err?.message || err)
  }
}

function localRegistry () {
  if (!globalThis.__dialtoneAutobaseKeys) {
    globalThis.__dialtoneAutobaseKeys = {}
  }
  return globalThis.__dialtoneAutobaseKeys
}

function loadLocalRegistry (topicName) {
  const registry = localRegistry()
  return typeof registry[topicName] === 'string' ? registry[topicName] : null
}

function saveLocalRegistry (topicName, keyHex) {
  const registry = localRegistry()
  registry[topicName] = keyHex
}

async function flushWithTimeout (target, timeoutMs, label) {
  let timeoutId
  const timeout = new Promise((resolve) => {
    timeoutId = setTimeout(() => {
      console.log(`${label} timed out after ${timeoutMs}ms`)
      resolve()
    }, timeoutMs)
  })
  const flushed = typeof target.flush === 'function'
    ? target.flush()
    : target.flushed()
  await Promise.race([flushed, timeout])
  clearTimeout(timeoutId)
}
