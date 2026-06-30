import { mount } from 'svelte'
import App from './App.svelte'

// Svelte 5 mounts components via mount(), not `new Component()`.
// The #app element exists because this module script runs after the body
// is parsed; App.svelte waits for the Wails runtime internally before it
// calls any go bindings, so mounting immediately is safe.
const app = mount(App, {
  target: document.getElementById('app'),
})

export default app
