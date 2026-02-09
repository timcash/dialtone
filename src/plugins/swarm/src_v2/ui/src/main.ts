import './style.css'

document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
  <div>
    <h1>Swarm V2 Dashboard</h1>
    <div class="card">
      <p>Node Status: <span id="status">Connecting...</span></p>
      <div id="logs"></div>
    </div>
  </div>
`

async function updateStatus() {
  try {
    const res = await fetch('/api/status')
    const data = await res.json()
    document.getElementById('status')!.textContent = data.status
  } catch (e) {
    document.getElementById('status')!.textContent = 'Offline'
  }
}

setInterval(updateStatus, 2000)
updateStatus()
