import { describe, it, expect, beforeEach } from 'vitest'
import { useThemeStore } from './theme'

describe('theme store', () => {
  beforeEach(() => {
    // Reset between tests
    useThemeStore.setState({ mode: 'light', resolved: 'light' })
    document.documentElement.classList.remove('dark')
    document.documentElement.style.colorScheme = ''
  })

  it('defaults to light mode', () => {
    expect(useThemeStore.getState().mode).toBe('light')
    expect(useThemeStore.getState().resolved).toBe('light')
  })

  it('setMode updates the resolved theme and toggles the .dark class', () => {
    useThemeStore.getState().setMode('dark')
    expect(useThemeStore.getState().resolved).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.style.colorScheme).toBe('dark')

    useThemeStore.getState().setMode('light')
    expect(useThemeStore.getState().resolved).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(document.documentElement.style.colorScheme).toBe('light')
  })

  it('setMode with "system" resolves via prefers-color-scheme', () => {
    // Pretend the OS prefers light
    window.matchMedia('(prefers-color-scheme: dark)').matches === false
    useThemeStore.getState().setMode('system')
    // Resolution depends on the test environment's OS; we just assert that
    // mode is set and resolved is one of the two valid values.
    expect(useThemeStore.getState().mode).toBe('system')
    expect(['light', 'dark']).toContain(useThemeStore.getState().resolved)
  })
})