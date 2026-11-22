import { createRemoteJWKSet, jwtVerify, type JWTVerifyGetKey } from 'jose';
import type { Principal } from '../app/principal';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export class Unauthenticated extends Error {}

/**
 * Valida tokens do Keycloak: assinatura (JWKS), emissor, audiência e expiração. O tenant e o
 * colaborador vêm só das claims `tenant_id` e `sub`; nada disso é aceito do corpo da requisição.
 */
export class TokenVerifier {
  constructor(
    private readonly issuer: string,
    private readonly audience: string,
    private readonly keys: JWTVerifyGetKey,
  ) {}

  static remote(issuer: string, audience: string, jwksUrl: string) {
    return new TokenVerifier(issuer, audience, createRemoteJWKSet(new URL(jwksUrl)));
  }

  async verify(token: string): Promise<Principal> {
    try {
      const { payload } = await jwtVerify(token, this.keys, {
        issuer: this.issuer,
        audience: this.audience,
        algorithms: ['RS256', 'ES256', 'PS256'],
      });
      const tenantId = payload['tenant_id'];
      if (typeof tenantId !== 'string' || !UUID.test(tenantId)) throw new Unauthenticated('claim tenant_id ausente ou inválida');
      if (!payload.sub || !UUID.test(payload.sub)) throw new Unauthenticated('claim sub inválida');
      const roles = (payload['realm_access'] as { roles?: string[] } | undefined)?.roles ?? [];
      const segment = payload['segment_key'];
      return { tenantId, employeeId: payload.sub, roles, segmentKey: typeof segment === 'string' ? segment : undefined };
    } catch (e) {
      if (e instanceof Unauthenticated) throw e;
      throw new Unauthenticated('token inválido');
    }
  }
}
