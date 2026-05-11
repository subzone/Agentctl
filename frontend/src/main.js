import App from './App.svelte'
import './wailsjs/runtime'

const app = new App({
  target: document.getElementById('app')
})

export default app
