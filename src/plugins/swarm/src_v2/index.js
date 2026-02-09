const mode = Pear.config.args[0]
if (mode === 'dashboard') {
  await import('./bare/dashboard.js')
} else {
  console.log('Swarm V2 Node active. Topic:', mode || 'default')
}
