import { forbidden } from '../domain/errors';

export interface Principal {
  tenantId: string;
  employeeId: string;
  roles: string[];
  /** Corte organizacional vindo do token (ex.: "obras/sao-paulo"); usado no clima. */
  segmentKey?: string;
}

export const hasRole = (p: Principal, ...roles: string[]) => roles.some((r) => p.roles.includes(r));

export function requireRole(p: Principal, ...roles: string[]): void {
  if (!hasRole(p, ...roles)) throw forbidden('sem permissão para esta operação');
}
