import Autobase from 'autobase'
import Corestore from 'corestore'
import Hyperswarm from 'hyperswarm'
import Hyperbee from 'hyperbee'
import b4a from 'b4a'
import crypto from 'hypercore-crypto'

export class AutoKV {
  constructor (opts = {}) {
    const { topic, storage, bootstrap = null } = opts
    this.store = new Corestore(storage)
    this.topic = crypto.hash(b4a.from(topic, 'utf8'))
    this.keyTopic = crypto.hash(b4a.from(topic + ':bootstrap', 'utf8'))
    this.bootstrap = bootstrap
    this.swarm = new Hyperswarm({ mdns: true })
    this.keySwarm = new Hyperswarm({ mdns: true })
  }

  async ready () {
    const core = this.store.get({ name: 'autokv' })
    await core.ready()
    if (!this.bootstrap) this.bootstrap = b4a.toString(core.key, 'hex')

    this.base = new Autobase(this.store, this.bootstrap, {
      valueEncoding: 'json',
      open: store => store.get('autokv', { valueEncoding: 'json' }),
      apply: async (nodes, view, host) => {
        const bee = new Hyperbee(view, { keyEncoding: 'utf-8', valueEncoding: 'json' })
        for (const { value } of nodes) {
          if (value.addWriter) await host.addWriter(b4a.from(value.addWriter, 'hex'))
          else if (value.key) await bee.put(value.key, value.value)
        }
      }
    })
    await this.base.ready()
    this.bee = new Hyperbee(this.base.view, { keyEncoding: 'utf-8', valueEncoding: 'json' })

    this.keySwarm.on('connection', s => {
      s.write(`BASE_KEY:${b4a.toString(this.base.key, 'hex')}\nWRITER_KEY:${b4a.toString(this.base.local.key, 'hex')}\n`)
      s.on('data', d => {
        const line = d.toString()
        if (line.startsWith('WRITER_KEY:')) this.base.append({ addWriter: line.split(':')[1].trim() })
      })
    })
    this.keySwarm.join(this.keyTopic, { server: true, client: true })
    this.swarm.on('connection', s => this.store.replicate(s))
    this.swarm.join(this.topic, { server: true, client: true })
  }

  async put (key, value) {
    await this.base.append({ key, value })
  }

  async get (key) {
    const node = await this.bee.get(key)
    return node ? node.value : null
  }

  async close () {
    await this.swarm.destroy()
    await this.keySwarm.destroy()
    await this.store.close()
  }
}