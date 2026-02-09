import { AutoLog } from './bare/autolog.js'
import { AutoKV } from './bare/autokv.js'
import path from 'bare-path'
import os from 'bare-os'

const args = Pear.config.args
const topicArg = args.indexOf('--topic')
const topicName = topicArg !== -1 ? args[topicArg + 1] : 'dialtone-default'
const isUi = args.includes('--ui')

console.log(`[swarm] Starting node on topic: ${topicName}`)
const baseDir = path.join(os.homedir(), '.dialtone', 'swarm', 'nodes', topicName)

const log = new AutoLog({
  topic: topicName + '-log',
  storage: path.join(baseDir, 'log')
})

const kv = new AutoKV({
  topic: topicName + '-kv',
  storage: path.join(baseDir, 'kv')
})

await log.ready()
await kv.ready()

if (isUi || args[0] === 'dashboard') {
  const { startDashboard } = await import('./bare/dashboard.js')
  startDashboard(log, kv)
} else {
  console.log('[swarm] Node active (no UI).')
}

Pear.teardown(async () => {
  await log.close()
  await kv.close()
})