import { INestApplication } from '@nestjs/common';
import { Test } from '@nestjs/testing';
import request from 'supertest';
import { AppModule } from '../src/app.module';
import { MemoryStore } from './support/memory-store';
import { ANA, BRUNO, depsFor, TENANT_A, TENANT_B } from './support/fixtures';
import { makeIssuer } from './support/tokens';

describe('API HTTP', () => {
  let app: INestApplication;
  let sign: Awaited<ReturnType<typeof makeIssuer>>['sign'];
  const token = (sub: string, tenant: string, roles: string[] = ['employee'], segment_key?: string) =>
    sign({ sub, tenant_id: tenant, realm_access: { roles }, ...(segment_key ? { segment_key } : {}) });

  beforeAll(async () => {
    const i = await makeIssuer();
    sign = i.sign;
    const mod = await Test.createTestingModule({
      imports: [AppModule.register({ deps: depsFor(new MemoryStore()), verifier: i.verifier })],
    }).compile();
    app = mod.createNestApplication();
    await app.init();
  });
  afterAll(() => app.close());

  it('sem token ou com token inválido responde 401 problem+json', async () => {
    const r = await request(app.getHttpServer()).get('/v1/feedbacks').expect(401);
    expect(r.headers['content-type']).toMatch(/problem\+json/);
    await request(app.getHttpServer()).get('/v1/feedbacks').set('Authorization', 'Bearer lixo').expect(401);
  });

  it('healthz é público', async () => {
    await request(app.getHttpServer()).get('/healthz').expect(200);
  });

  it('feedback: o tenant vem do token e o corpo é validado (campos desconhecidos, tenant_id forjado)', async () => {
    const auth = `Bearer ${await token(ANA, TENANT_A)}`;
    await request(app.getHttpServer()).post('/v1/feedbacks').set('Authorization', auth)
      .send({ to_employee_id: BRUNO, body: 'Bom trabalho', tenant_id: TENANT_B }).expect(422);
    const ok = await request(app.getHttpServer()).post('/v1/feedbacks').set('Authorization', auth)
      .send({ to_employee_id: BRUNO, body: 'Bom trabalho' }).expect(201);
    expect(ok.body).toMatchObject({ from_employee_id: ANA, to_employee_id: BRUNO });
    // o destinatário vê; o mesmo colaborador em outro tenant não
    await request(app.getHttpServer()).get('/v1/feedbacks').set('Authorization', `Bearer ${await token(BRUNO, TENANT_A)}`).expect(200).expect((r) => expect(r.body).toHaveLength(1));
    await request(app.getHttpServer()).get('/v1/feedbacks').set('Authorization', `Bearer ${await token(BRUNO, TENANT_B)}`).expect(200).expect((r) => expect(r.body).toHaveLength(0));
  });

  it('clima: fluxo de criação, resposta anônima e resultados com permissões', async () => {
    const admin = `Bearer ${await token(ANA, TENANT_A, ['tenant-admin'])}`;
    const created = await request(app.getHttpServer()).post('/v1/surveys').set('Authorization', admin)
      .send({ title: 'Clima', questions: [{ id: 'q1', text: 'ok?' }], segments: ['vendas'] }).expect(201);
    const id = created.body.id;
    const eu = `Bearer ${await token(BRUNO, TENANT_A, ['employee'], 'vendas')}`;
    await request(app.getHttpServer()).post(`/v1/surveys/${id}/responses`).set('Authorization', eu).send({ scores: { q1: 5 } }).expect(202);
    await request(app.getHttpServer()).post(`/v1/surveys/${id}/responses`).set('Authorization', eu).send({ scores: { q1: 5 } }).expect(409);
    await request(app.getHttpServer()).get(`/v1/surveys/${id}/results`).set('Authorization', eu).expect(403);
    const res = await request(app.getHttpServer()).get(`/v1/surveys/${id}/results`).set('Authorization', admin).expect(200);
    expect(res.body).toMatchObject({ min_group_size: 5, participation: 1, segments: [] });
    await request(app.getHttpServer()).get(`/v1/surveys/${id}/results`).set('Authorization', `Bearer ${await token(ANA, TENANT_B, ['tenant-admin'])}`).expect(404);
  });

  it('confirmação de leitura responde 204', async () => {
    const gestor = `Bearer ${await token(ANA, TENANT_A, ['manager'])}`;
    const post = await request(app.getHttpServer()).post('/v1/posts').set('Authorization', gestor).send({ title: 'Aviso', body: 'Texto' }).expect(201);
    await request(app.getHttpServer()).post(`/v1/posts/${post.body.id}/read-receipts`).set('Authorization', `Bearer ${await token(BRUNO, TENANT_A)}`).expect(204);
    await request(app.getHttpServer()).get(`/v1/posts/${post.body.id}/read-receipts`).set('Authorization', gestor).expect(200).expect((r) => expect(r.body).toHaveLength(1));
    await request(app.getHttpServer()).post('/v1/posts/não-é-uuid/read-receipts').set('Authorization', gestor).expect(400);
  });
});
