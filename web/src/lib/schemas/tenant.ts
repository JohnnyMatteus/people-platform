import { z } from 'zod'

/** Espelho de Branding em contracts/openapi/tenant-service.yaml. */
export const branding = z.object({
  tenant_id: z.string().uuid(),
  slug: z.string(),
  name: z.string(),
  tokens: z.record(z.string(), z.string()),
  logo_url: z.string().optional(),
})
export type Branding = z.infer<typeof branding>
