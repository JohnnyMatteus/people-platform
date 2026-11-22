import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import Ajv2020 from 'ajv/dist/2020';
import addFormats from 'ajv-formats';
import { SocialService } from '../src/app/social-service';
import { SurveyService } from '../src/app/survey-service';
import { toEnvelope } from '../src/domain/events';
import { ANA, BRUNO, depsFor, principal, TENANT_A } from './support/fixtures';
import { MemoryStore } from './support/memory-store';

const schemaDir = join(__dirname, '..', '..', '..', 'contracts', 'asyncapi', 'schemas');
const load = (f: string) => JSON.parse(readFileSync(join(schemaDir, f), 'utf8'));

function validator(file: string) {
  const ajv = new Ajv2020({ strict: false });
  addFormats(ajv);
  ajv.addSchema(load('envelope.v1.json'));
  return ajv.compile(load(file));
}

describe('eventos publicados respeitam os schemas do AsyncAPI', () => {
  it('ElogioPublicado', async () => {
    const store = new MemoryStore();
    await new SocialService(depsFor(store)).praise(principal(ANA), [BRUNO], 'Valeu', ['colaboracao']);
    const validate = validator('elogio-publicado.v1.json');
    const env = toEnvelope(store.outbox[0]!);
    expect(validate(env)).toBe(true);
    expect(validate({ ...env, data: { ...env.data, employee_name: 'x' } })).toBe(false);
  });

  it('PesquisaClimaEncerrada', async () => {
    const store = new MemoryStore();
    const d = depsFor(store);
    const survey = new SurveyService(d, (a) => a);
    const admin = principal(ANA, TENANT_A, ['tenant-admin']);
    const s = await survey.create(admin, { title: 'Clima', questions: [{ id: 'q1', text: 'ok' }], segments: ['vendas'] });
    for (let i = 0; i < 5; i++) await survey.answer(principal(crypto.randomUUID(), TENANT_A, [], 'vendas'), s.id, { q1: 4 });
    await survey.close(admin, s.id);
    expect(validator('pesquisa-clima-encerrada.v1.json')(toEnvelope(store.outbox[0]!))).toBe(true);
  });
});
