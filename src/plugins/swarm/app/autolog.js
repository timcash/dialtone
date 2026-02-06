import Autobase from 'autobase'
import b4a from 'b4a'

export class AutoLog {
  constructor (store, bootstrap = null) {
    this.base = new Autobase(store, bootstrap, {
      valueEncoding: 'json',
      open: openLog,
      apply: applyLog
    })
  }

  async ready () {
    await this.base.ready()
  }

  async addWriter (writerKey) {
    const key = normalizeWriterKey(writerKey)
    await this.base.append({ addWriter: b4a.toString(key, 'hex') })
    await this.base.update()
  }

  async append (event) {
    await this.base.append({ type: 'session', event })
    await this.base.update()
  }

  async list () {
    const entries = []
    const log = this.base.view || this.base._viewStore.get('autolog', { valueEncoding: 'json' })
    for (let i = 0; i < log.length; i++) {
      entries.push(await log.get(i))
    }
    return entries
  }
}

function openLog (store) {
  return store.get('autolog', { valueEncoding: 'json' })
}

async function applyLog (nodes, view, host) {
  for (const node of nodes) {
    const value = node.value
    if (value && value.addWriter) {
      const key = b4a.from(value.addWriter, 'hex')
      await host.addWriter(key, { indexer: true })
      continue
    }
    if (!value || value.type !== 'session' || !value.event) continue
    await view.append(value.event)
  }
}

function normalizeWriterKey (writerKey) {
  if (b4a.isBuffer(writerKey)) return writerKey
  if (typeof writerKey === 'string') return b4a.from(writerKey, 'hex')
  throw new Error('writerKey must be a hex string or buffer')
}
