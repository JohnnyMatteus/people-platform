import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import Ajv2020 from 'ajv/dist/2020';
import addFormats from 'ajv-formats';
import { SurveyService } from '../src/app/survey-service';
import { TenantProvisioningService } from '../src/app/tenant-provisioning-service';
import { MemoryStore } from './support/memory-store';
import { ANA, depsFor, principal, tenantCriado, TENANT_A, TENANT_B } from './support/fixtures';

const schemaDir = join(__dirname, '..', '..', '..', 'contracts', 'asyncapi', 'schemas');
const load = (f: string) => JSON.parse(readFileSync(join(schemaDir, f), 'utf8'));

const setup = () => {
  const store = new MemoryStore();
  const d = depsFor(store);
  return { store, provisioning: new TenantProvisioningService(d), survey: new SurveyService(d) };
};

describe('preparo do tenant por TenantCriado', () => {
  it('o fixture respeita o schema do contrato', () => {
    const ajv = addFormats(new Ajv2020({ strict: false }));
    ajv.addSchema(load('envelope.v1.json'));
    const validate = ajv.compile(load('tenant-criado.v1.json'));
    const env = { ...tenantCriado(TENANT_A, ['climate_survey']), aggregate_id: TENANT_A, occurred_at: '2026-11-22T10:00:00Z', producer: 'tenant-service' };
    expect(validate(env)).toBe(true);
  });

  it('grava as configurações do módulo uma única vez e isola por tenant', async () => {
    const { store, provisioning } = setup();
    const env = tenantCriado(TENANT_A, ['climate_survey']);
    expect(await provisioning.onTenantCreated(env)).toBe(true);
    expect(await provisioning.onTenantCreated(env)).toBe(false);
    expect(store.settings).toEqual([{ tenantId: TENANT_A, slug: 'acme', name: 'Acme', plan: 'crescimento', climateEnabled: true, minGroupSize: 5 }]);
    await provisioning.onTenantCreated(tenantCriado(TENANT_B, []));
    expect(store.settings.find((s) => s.tenantId === TENANT_B)?.climateEnabled).toBe(false);
  });

  it('a pesquisa de clima herda o tamanho mínimo de grupo do tenant', async () => {
    const { store, provisioning, survey } = setup();
    await provisioning.onTenantCreated(tenantCriado(TENANT_A, ['climate_survey']));
    store.settings[0]!.minGroupSize = 8;
    const s = await survey.create(principal(ANA, TENANT_A, ['tenant-admin']), { title: 'Clima', questions: [{ id: 'q1', text: 'ok' }], segments: ['vendas'] });
    expect(s.minGroupSize).toBe(8);
  });
});
