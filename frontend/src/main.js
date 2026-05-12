import App from './App.svelte'

let app

function mount() {
  app = new App({
    target: document.getElementById('app')
  })
}

// Wails injects window.runtime after the page loads.
// Wait for it before mounting so all go bindings are available.
if (window.runtime) {
  mount()
} else {
  window.addEventListener('DOMContentLoaded', () => {
    // Give Wails a tick to inject runtime
    setTimeout(mount, 0)
  })
}

export default app
