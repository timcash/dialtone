import Autobase from 'autobase'
import Hyperbee from 'hyperbee'
import b4a from 'b4a'

export class AutoKV {
  constructor (store, bootstrap = null) {
    this.base = new Autobase(store, bootstrap, {
      valueEncoding: 'json',
      open: openKv,
      apply: applyKv
    })
    this.bee = null
  }

  async ready () {
    await this.base.ready()
    const view = this.base.view || this.base._viewStore.get('autokv', { valueEncoding: 'json' })
    this.bee = new Hyperbee(view, {
      extension: false,
      keyEncoding: 'utf-8',
      valueEncoding: 'json'
    })
    await this.bee.ready()
  }

  async addWriter (writerKey) {
    const key = normalizeWriterKey(writerKey)
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
