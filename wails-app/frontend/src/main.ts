import { mount } from 'svelte'
import './style.css'
import App from './App.svelte'
import { loadSettings } from './lib/settings'

const target = document.getElementById('app')
if (!target) {
  throw new Error('Elemento #app não encontrado no documento.')
}

void loadSettings().then(() => mount(App, { target }))
