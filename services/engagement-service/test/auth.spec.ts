import { AUDIENCE, ISSUER, makeIssuer } from './support/tokens';
import { ANA, TENANT_A } from './support/fixtures';

describe('validação de JWT do Keycloak', () => {
  const claims = { sub: ANA, tenant_id: TENANT_A, realm_access: { roles: ['manager'] }, segment_key: 'obras/sp' };

  it('aceita token válido e extrai tenant, colaborador, papéis e segmento', async () => {
    const { verifier, sign } = await makeIssuer();
    expect(await verifier.verify(await sign(claims))).toEqual({ tenantId: TENANT_A, employeeId: ANA, roles: ['manager'], segmentKey: 'obras/sp' });
  });

  it.each([
    ['expirado', (s: Awaited<ReturnType<typeof makeIssuer>>) => s.sign(claims, { exp: Math.floor(Date.now() / 1000) - 60 })],
    ['emissor errado', (s: Awaited<ReturnType<typeof makeIssuer>>) => s.sign(claims, { issuer: 'http://evil.example.com' })],
    ['audiência errada', (s: Awaited<ReturnType<typeof makeIssuer>>) => s.sign(claims, { audience: 'outra-api' })],
    ['assinado com outra chave', (s: Awaited<ReturnType<typeof makeIssuer>>) => s.sign(claims, { key: s.otherKey })],
    ['sem tenant_id', (s: Awaited<ReturnType<typeof makeIssuer>>) => s.sign({ sub: ANA })],
    ['tenant_id que não é UUID', (s: Awaited<ReturnType<typeof makeIssuer>>) => s.sign({ ...claims, tenant_id: 'abc' })],
    ['sub que não é UUID', (s: Awaited<ReturnType<typeof makeIssuer>>) => s.sign({ ...claims, sub: 'ana' })],
  ])('recusa token %s', async (_, make) => {
    const i = await makeIssuer();
    await expect(i.verifier.verify(await make(i))).rejects.toThrow();
  });

  it('recusa alg=none', async () => {
    const { verifier } = await makeIssuer();
    const b64 = (o: object) => Buffer.from(JSON.stringify(o)).toString('base64url');
    const token = `${b64({ alg: 'none' })}.${b64({ ...claims, iss: ISSUER, aud: AUDIENCE })}.`;
    await expect(verifier.verify(token)).rejects.toThrow();
  });
});
