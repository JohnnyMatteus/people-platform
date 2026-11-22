import { FetchError, ofetch } from 'ofetch'
import type { ZodType } from 'zod'
import { config } from './config'
import { session } from './auth'

/** Erro no formato RFC 9457 devolvido pelos serviços. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
    readonly type?: string,
  ) {
    super(message)
  }
}

const client = ofetch.create({
  baseURL: config.apiBase,
  onRequest({ options }) {
    const token = session.accessToken()
    if (token) {
      const headers = new Headers(options.headers)
      headers.set('Authorization', `Bearer ${token}`)
      options.headers = headers
    }
  },
})

type Method = 'GET' | 'POST' | 'PUT' | 'PATCH'

/** Toda resposta é validada com o schema zod do contrato antes de chegar à tela. */
export async function api<T>(schema: ZodType<T>, method: Method, path: string, body?: unknown): Promise<T> {
  try {
    const raw = await client(path, { method, body: body as Record<string, unknown> | undefined })
    return schema.parse(raw)
  } catch (e) {
    if (e instanceof FetchError) {
      const problem = e.data as { detail?: string; title?: string; type?: string } | undefined
      throw new ApiError(e.statusCode ?? 0, problem?.detail ?? problem?.title ?? 'Serviço indisponível', problem?.type)
    }
    throw e
  }
}

export const get = <T>(schema: ZodType<T>, path: string) => api(schema, 'GET', path)
export const post = <T>(schema: ZodType<T>, path: string, body?: unknown) => api(schema, 'POST', path, body)
export const put = <T>(schema: ZodType<T>, path: string, body?: unknown) => api(schema, 'PUT', path, body)
export const patch = <T>(schema: ZodType<T>, path: string, body?: unknown) => api(schema, 'PATCH', path, body)

export function errorMessage(e: unknown): string {
  if (e instanceof ApiError) return e.message
  if (e instanceof Error) return e.message
  return 'Algo deu errado.'
}
