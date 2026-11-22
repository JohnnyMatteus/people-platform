import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

// Toda rota que o front chama precisa existir no gateway (que por sua vez é conferido contra o OpenAPI de cada
// serviço). Varre todos os .ts e .vue de src, não só os queries.ts, para pegar chamada escrita direto numa tela.
const root = join(__dirname, '..')
const gatewayFile = join(__dirname, '..', '..', '..', 'infra', 'gateway', 'krakend.json')

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name)
    if (statSync(path).isDirectory()) return sources(path)
    return /\.(ts|vue)$/.test(name) && !name.endsWith('.test.ts') && !name.endsWith('.d.ts') ? [path] : []
  })
}

const normalize = (path: string) => path.replace(/\$\{[^}]+\}/g, '{}').replace(/\{[^}/]+\}/g, '{}').replace(/\?.*$/, '')

const gateway = new Set(
  (JSON.parse(readFileSync(gatewayFile, 'utf8')).endpoints as { method: string; endpoint: string }[]).map(
    (e) => `${e.method} ${normalize(e.endpoint)}`,
  ),
)

const helper: Record<string, string> = { get: 'GET', post: 'POST', put: 'PUT', patch: 'PATCH', del: 'DELETE' }

export function calls(text: string): string[] {
  const base = /const base = '([^']+)'/.exec(text)?.[1] ?? ''
  const found: string[] = []
  const add = (method: string, path: string) => found.push(`${method} ${normalize(path.replace('${base}', base))}`)
  for (const m of text.matchAll(/\b(get|post|put|patch|del)\(\s*[\w.]+,\s*[`']([^`']+)[`']/g)) add(helper[m[1] ?? ''] ?? '', m[2] ?? '')
  for (const m of text.matchAll(/\bapi\(\s*[\w.]+,\s*'(GET|POST|PUT|PATCH|DELETE)',\s*[`']([^`']+)[`']/g)) add(m[1] ?? '', m[2] ?? '')
  return found
}

describe('contrato front x gateway', () => {
  const all = sources(root).flatMap((f) => calls(readFileSync(f, 'utf8')))

  it('encontra as chamadas do front', () => {
    expect(all.length).toBeGreaterThan(10)
  })

  it('reconhece DELETE, o helper api e a query string', () => {
    expect(calls("del(s, `/tenant/v1/tenants/${id}`)\napi(s, 'GET', `/analytics/v1/analytics/turnover?x=${y}`)")).toEqual([
      'DELETE /tenant/v1/tenants/{}',
      'GET /analytics/v1/analytics/turnover',
    ])
  })

  it.each([...new Set(all)])('%s existe no gateway', (call) => {
    expect(gateway.has(call)).toBe(true)
  })
})
