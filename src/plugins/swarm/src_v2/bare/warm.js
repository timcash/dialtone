import { AutoLog } from './autolog.js'
import { AutoKV } from './autokv.js'
import path from 'bare-path'
import os from 'bare-os'

async function main() {
  const topic = Bare.argv[Bare.argv.length - 1] || 'dialtone-warm'
  const baseDir = path.join(os.homedir(), '.dialtone', 'swarm', 'warm', topic)
  
  const log = new AutoLog({ topic: topic + '-log', storage: path.join(baseDir, 'log') })
  const kv = new AutoKV({ topic: topic + '-kv', storage: path.join(baseDir, 'kv') })

  await log.ready()
  await kv.ready()

  console.log(`Peer is now ACTIVE for topic: ${topic}`)
}

main().catch(console.error)
