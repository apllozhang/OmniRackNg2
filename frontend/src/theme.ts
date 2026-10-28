import { ref } from 'vue'
export const darkTheme = ref(false)
export function initializeTheme() {
  let saved: string | null = null
  try { saved = localStorage.getItem('omnirack-theme') } catch { /* private browsing */ }
  darkTheme.value = saved ? saved === 'dark' : Boolean(window.matchMedia?.('(prefers-color-scheme: dark)').matches)
  applyTheme()
}
function applyTheme() {
  document.documentElement.dataset.theme = darkTheme.value ? 'dark' : 'light'
  document.documentElement.style.colorScheme = darkTheme.value ? 'dark' : 'light'
}
export function toggleTheme() {
  darkTheme.value = !darkTheme.value
  applyTheme()
  try { localStorage.setItem('omnirack-theme', darkTheme.value ? 'dark' : 'light') } catch { /* state remains usable */ }
}
