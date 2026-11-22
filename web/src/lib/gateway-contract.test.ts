import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

// Toda rota que o front chama precisa existir no gateway (que por sua vez é conferido contra o OpenAPI de cada serviço).
const root = join(__dirname, '..')
const gatewayFile = join(__dirname, '..', '..', '..', 'infra', 'gateway', 'krakend.json')

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name)
    if (statSync(path).isDirectory()) return sources(path)
    return name === 'queries.ts' ? [path] : []
  })
}

const normalize = (path: string) => path.replace(/\$\{[^}]+\}/g, '{}').replace(/\{[^}/]+\}/g, '{}')

const gateway = new Set(
  (JSON.parse(readFileSync(gatewayFile, 'utf8')).endpoints as { method: string; endpoint: string }[]).map(
    (e) => `${e.method} ${normalize(e.endpoint)}`,
  ),
)

function calls(file: string): string[] {
  const text = readFileSync(file, 'utf8')
  const base = /const base = '([^']+)'/.exec(text)?.[1] ?? ''
  const found: string[] = []
  for (const m of text.matchAll(/\b(get|post|put|patch)\(\s*[\w.]+,\s*[`']([^`']+)[`']/g)) {
    const path = (m[2] ?? '').replace('${base}', base)
    found.push(`${(m[1] ?? '').toUpperCase()} ${normalize(path)}`)
  }
  return found
}

describe('contrato front x gateway', () => {
  const all = sources(root).flatMap(calls)

  it('encontra as chamadas das queries', () => {
    expect(all.length).toBeGreaterThan(10)
  })

  it.each(all)('%s existe no gateway', (call) => {
    expect(gateway.has(call)).toBe(true)
  })
})
