import { loadConfig } from '../src/platform/config';

describe('configuração', () => {
  it('falha cedo listando as variáveis ausentes', () => {
    expect(() => loadConfig({})).toThrow(/DATABASE_URL.*KAFKA_BROKERS/);
  });

  it('carrega com os padrões', () => {
    const c = loadConfig({
      DATABASE_URL: 'postgres://x', KAFKA_BROKERS: 'a:9092, b:9092',
      OIDC_ISSUER: 'http://kc/realms/people', OIDC_JWKS_URL: 'http://kc/certs',
    });
    expect(c.kafkaBrokers).toEqual(['a:9092', 'b:9092']);
    expect(c.OIDC_AUDIENCE).toBe('people-api');
  });
});
