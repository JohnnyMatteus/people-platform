import { createLocalJWKSet, exportJWK, generateKeyPair, SignJWT, type JWTPayload, type KeyLike } from 'jose';
import { TokenVerifier } from '../../src/platform/auth';

export const ISSUER = 'http://localhost:8180/realms/people';
export const AUDIENCE = 'people-api';

export async function makeIssuer() {
  const { publicKey, privateKey } = await generateKeyPair('RS256');
  const jwk = { ...(await exportJWK(publicKey)), kid: 'k1', alg: 'RS256', use: 'sig' };
  const verifier = new TokenVerifier(ISSUER, AUDIENCE, createLocalJWKSet({ keys: [jwk] }));
  const sign = (claims: JWTPayload, o: { issuer?: string; audience?: string; exp?: string | number; key?: KeyLike } = {}) =>
    new SignJWT(claims)
      .setProtectedHeader({ alg: 'RS256', kid: 'k1' })
      .setIssuer(o.issuer ?? ISSUER)
      .setAudience(o.audience ?? AUDIENCE)
      .setIssuedAt()
      .setExpirationTime(o.exp ?? '5m')
      .sign(o.key ?? privateKey);
  return { verifier, sign, otherKey: (await generateKeyPair('RS256')).privateKey };
}
