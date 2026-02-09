import { spawn } from 'bare-subprocess'
import fs from 'bare-fs'
import path from 'bare-path'
import os from 'bare-os'

async function main() {
  console.log('--- Test 1: Warm Node Persistence ---')
  
  // 1. First Pass: Start or Detect
  console.log('[test] Phase 1: Initial startup/detection...')
  const startedFirst = await ensureWarmNode()
  console.log(startedFirst ? '[test] Phase 1: Started new node.' : '[test] Phase 1: Detected existing node.')

  // 2. Second Pass: Must Skip
  console.log('[test] Phase 2: Verifying idempotency (second call should skip)...')
  const startedSecond = await ensureWarmNode()
  
  if (startedSecond) {
    console.error('[FAIL] Test 1: Idempotency failure. Second call attempted to start a new node.')
    Bare.exit(1)
  }

  console.log('[test] Phase 2: Correctly skipped startup.')
  console.log('--- [PASS] TEST 1 SUCCESS ---')
  Bare.exit(0)
}

async function ensureWarmNode() {
  const topic = 'dialtone-v2'
  const isRunning = await checkWarmNode()

  if (isRunning) {
    return false // Skipped
  }

  const logPath = 'warm_stdout.log'
  if (fs.existsSync(logPath)) fs.unlinkSync(logPath)

  const cmd = 'pear'
  const args = ['run', 'warm.js', topic]
  
  const stdout = fs.openSync('warm_stdout.log', 'w')
  const stderr = fs.openSync('warm_stderr.log', 'w')
  
  const proc = spawn(cmd, args, {
    detached: true,
    stdio: ['ignore', stdout, stderr]
  })
  
  proc.unref()
  
  // Wait for it to become active in logs
  const start = Date.now()
  while (Date.now() - start < 15000) {
    if (fs.existsSync(logPath)) {
      const content = fs.readFileSync(logPath, 'utf8')
      if (content.includes('Peer is now ACTIVE')) {
        return true // Started
      }
    }
    await new Promise(r => setTimeout(r, 1000))
  }
  
  throw new Error('Timeout waiting for warm node to become active.')
}

async function checkWarmNode() {
  return new Promise((resolve) => {
    try {
      const ps = spawn('ps', ['aux'])
      let output = ''
      ps.stdout.on('data', (data) => { output += data.toString() })
      ps.on('exit', () => {
        // Look for warm.js specifically
        resolve(output.includes('warm.js'))
      })
    } catch {
      resolve(false)
    }
  })
}

main()
