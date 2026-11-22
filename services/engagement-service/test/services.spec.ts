import { randomUUID } from 'node:crypto';
import { GamificationService } from '../src/app/gamification-service';
import { OneOnOneService } from '../src/app/one-on-one-service';
import { SocialService } from '../src/app/social-service';
import { SurveyService } from '../src/app/survey-service';
import { toEnvelope } from '../src/domain/events';
import { MemoryStore } from './support/memory-store';
import { ANA, BRUNO, CARLA, depsFor, principal, TENANT_A, TENANT_B } from './support/fixtures';

const setup = () => {
  const store = new MemoryStore();
  const d = depsFor(store);
  return { store, social: new SocialService(d), oneOnOne: new OneOnOneService(d), survey: new SurveyService(d, (a) => a), gamification: new GamificationService(d) };
};

describe('feedback privado', () => {
  it('só as duas partes enxergam, com histórico', async () => {
    const { social } = setup();
    await social.giveFeedback(principal(ANA), BRUNO, 'Ótima entrega');
    await social.giveFeedback(principal(BRUNO), ANA, 'Obrigado');
    expect(await social.listFeedbacks(principal(ANA), BRUNO)).toHaveLength(2);
    expect(await social.listFeedbacks(principal(CARLA))).toHaveLength(0);
    expect(await social.listFeedbacks(principal(CARLA), ANA)).toHaveLength(0);
  });

  it('recusa feedback para si mesmo', async () => {
    await expect(setup().social.giveFeedback(principal(ANA), ANA, 'oi')).rejects.toThrow();
  });
});

describe('elogio e gamificação por evento', () => {
  it('o elogio grava ElogioPublicado na outbox e o consumo credita pontos e badge, uma vez só', async () => {
    const { store, social, gamification } = setup();
    await social.praise(principal(ANA), [BRUNO], 'Parabéns!', ['colaboracao']);
    expect(store.outbox).toHaveLength(1);
    const env = toEnvelope(store.outbox[0]!);
    expect(env.event_type).toBe('ElogioPublicado');
    expect(env.data).toMatchObject({ from_employee_id: ANA, to_employee_ids: [BRUNO], values: ['colaboracao'] });

    await gamification.onPraisePublished(env as never);
    await gamification.onPraisePublished(env as never); // reentrega do Kafka
    expect((await gamification.mine(principal(BRUNO))).balance).toBe(10);
    expect((await gamification.mine(principal(ANA))).balance).toBe(5);
    expect((await gamification.mine(principal(BRUNO))).badges.map((b) => b.code)).toEqual(['primeiro-reconhecimento']);
  });

  it('badge de "reconhecido" chega com 10 elogios recebidos', async () => {
    const { gamification } = setup();
    for (let i = 0; i < 10; i++) {
      await gamification.onPraisePublished({ event_id: randomUUID(), event_type: 'ElogioPublicado', tenant_id: TENANT_A, data: { from_employee_id: ANA, to_employee_ids: [BRUNO] } });
    }
    const codes = (await gamification.mine(principal(BRUNO))).badges.map((b) => b.code);
    expect(codes).toEqual(expect.arrayContaining(['primeiro-reconhecimento', 'reconhecido']));
    expect((await gamification.mine(principal(ANA))).badges.map((b) => b.code)).toContain('generoso');
    expect((await gamification.leaderboard(principal(ANA)))[0]).toEqual({ employeeId: BRUNO, points: 100 });
  });

  it('pontos de um tenant não aparecem no outro', async () => {
    const { gamification } = setup();
    await gamification.onPraisePublished({ event_id: randomUUID(), event_type: 'ElogioPublicado', tenant_id: TENANT_A, data: { from_employee_id: ANA, to_employee_ids: [BRUNO] } });
    expect((await gamification.mine(principal(BRUNO, TENANT_B))).balance).toBe(0);
    expect(await gamification.leaderboard(principal(BRUNO, TENANT_B))).toEqual([]);
  });
});

describe('1:1 com pauta e contexto', () => {
  it('participantes montam a pauta; o contexto traz feedbacks e elogios entre eles', async () => {
    const { social, oneOnOne } = setup();
    const lider = principal(ANA, TENANT_A, ['manager']);
    const m = await oneOnOne.schedule(lider, BRUNO, new Date('2026-12-01T14:00:00Z'));
    const withItem = await oneOnOne.addItem(principal(BRUNO), m.id, 'Plano de carreira');
    await oneOnOne.completeItem(lider, m.id, withItem.agenda[0]!.id);
    await oneOnOne.addItem(lider, m.id, 'Meta do trimestre');
    await social.giveFeedback(lider, BRUNO, 'Bom ritmo');
    await social.praise(principal(BRUNO), [ANA], 'Obrigado pelo apoio', []);

    const ctx = await oneOnOne.context(principal(BRUNO), m.id);
    expect(ctx.recentFeedbacks).toHaveLength(1);
    expect(ctx.recentPraises).toHaveLength(1);
    expect(ctx.openAgenda.map((i) => i.text)).toEqual(['Meta do trimestre']);
  });

  it('quem não participa recebe 404, também para a pauta e o contexto', async () => {
    const { oneOnOne } = setup();
    const m = await oneOnOne.schedule(principal(ANA), BRUNO, new Date());
    await expect(oneOnOne.context(principal(CARLA), m.id)).rejects.toMatchObject({ code: 'not_found' });
    await expect(oneOnOne.addItem(principal(CARLA), m.id, 'x')).rejects.toMatchObject({ code: 'not_found' });
  });
});

describe('rede social com confirmação de leitura', () => {
  it('só gestor publica; a leitura é idempotente, pontua uma vez e o autor vê quem leu', async () => {
    const { social, gamification } = setup();
    await expect(social.publishPost(principal(ANA), 't', 'b')).rejects.toMatchObject({ code: 'forbidden' });
    const gestor = principal(ANA, TENANT_A, ['manager']);
    const post = await social.publishPost(gestor, 'Novo benefício', 'Detalhes');
    await social.confirmRead(principal(BRUNO), post.id);
    await social.confirmRead(principal(BRUNO), post.id);
    const [view] = await social.listPosts(principal(BRUNO));
    expect(view).toMatchObject({ readCount: 1, readByMe: true });
    expect((await gamification.mine(principal(BRUNO))).balance).toBe(1);
    expect(await social.receipts(gestor, post.id)).toHaveLength(1);
    await expect(social.receipts(principal(BRUNO), post.id)).rejects.toMatchObject({ code: 'forbidden' });
  });
});

describe('clima anônimo: provas de anonimato', () => {
  const admin = principal(ANA, TENANT_A, ['tenant-admin']);
  const open = async (svc: SurveyService, k?: number) =>
    svc.create(admin, { title: 'Clima 2026', questions: [{ id: 'q1', text: 'Estou bem' }], segments: ['obras', 'obras/sp', 'vendas'], minGroupSize: k });
  const employee = (n: number, seg: string) => principal(randomUUID(), TENANT_A, ['employee'], seg);

  it('nada gravado na resposta aponta para o colaborador', async () => {
    const { store, survey } = setup();
    const s = await open(survey);
    const eu = principal(BRUNO, TENANT_A, ['employee'], 'vendas');
    await survey.answer(eu, s.id, { q1: 4 }, 'Tudo bem, fale com bruno@example.com');
    expect(store.responses).toHaveLength(1);
    const stored = JSON.stringify(store.responses);
    expect(stored).not.toContain(BRUNO);
    expect(stored).not.toContain('bruno@example.com');
    expect(Object.keys(store.responses[0]!)).not.toEqual(expect.arrayContaining(['employeeId']));
    // a participação existe, mas só guarda quem participou, sem nota nem segmento
    expect(store.participation).toEqual([{ tenantId: TENANT_A, surveyId: s.id, employeeId: BRUNO, on: '2026-11-21' }]);
    expect(Object.keys(store.participation[0]!)).not.toEqual(expect.arrayContaining(['segmentKey', 'scores']));
  });

  it('não dá para responder duas vezes', async () => {
    const { survey } = setup();
    const s = await open(survey);
    const eu = principal(BRUNO, TENANT_A, ['employee'], 'vendas');
    await survey.answer(eu, s.id, { q1: 4 });
    await expect(survey.answer(eu, s.id, { q1: 1 })).rejects.toMatchObject({ code: 'conflict' });
  });

  it('o segmento vem do token, não do corpo; sem segmento no token a resposta é recusada', async () => {
    const { survey } = setup();
    const s = await open(survey);
    await expect(survey.answer(principal(BRUNO), s.id, { q1: 4 })).rejects.toMatchObject({ code: 'invalid' });
  });

  it('resultados só saem com k atingido; funcionário comum não vê resultados', async () => {
    const { survey } = setup();
    const s = await open(survey);
    for (let i = 0; i < 4; i++) await survey.answer(employee(i, 'vendas'), s.id, { q1: 5 });
    expect((await survey.results(admin, s.id)).segments).toEqual([]);
    await survey.answer(employee(5, 'vendas'), s.id, { q1: 3 });
    const res = await survey.results(admin, s.id);
    expect(res.segments.map((x) => x.segmentKey)).toEqual(['_all', 'vendas']);
    expect(res.participation).toBe(5);
    await expect(survey.results(principal(BRUNO), s.id)).rejects.toMatchObject({ code: 'forbidden' });
  });

  it('encerrar publica PesquisaClimaEncerrada só com agregados, sem comentários nem identidade', async () => {
    const { store, survey } = setup();
    const s = await open(survey, 5);
    for (let i = 0; i < 5; i++) await survey.answer(employee(i, 'vendas'), s.id, { q1: 4 }, 'comentário livre');
    await survey.close(admin, s.id);
    const [ev] = store.outbox;
    expect(ev?.eventType).toBe('PesquisaClimaEncerrada');
    expect(ev?.payload).toEqual({
      survey_id: s.id,
      min_group_size: 5,
      segments: [
        { segment_key: '_all', respondents: 5, scores: { q1: 4 } },
        { segment_key: 'vendas', respondents: 5, scores: { q1: 4 } },
      ],
    });
    await expect(survey.answer(employee(9, 'vendas'), s.id, { q1: 4 })).rejects.toMatchObject({ code: 'conflict' });
  });

  it('a listagem traz as pesquisas do tenant com a participação de quem chama', async () => {
    const { survey } = setup();
    const s = await open(survey, 3);
    await survey.answer(principal(BRUNO, TENANT_A, [], 'obras'), s.id, { q1: 4 });
    const mine = await survey.list(principal(BRUNO, TENANT_A, [], 'obras'));
    const other = await survey.list(principal(CARLA, TENANT_A, [], 'obras'));
    expect(mine.map((x) => [x.id, x.alreadyAnswered])).toEqual([[s.id, true]]);
    expect(other.map((x) => x.alreadyAnswered)).toEqual([false]);
  });

  it('só tenant-admin cria e encerra; k abaixo de 3 é recusado', async () => {
    const { survey } = setup();
    await expect(survey.create(principal(BRUNO), { title: 't', questions: [{ id: 'q', text: 't' }], segments: ['a'] })).rejects.toMatchObject({ code: 'forbidden' });
    await expect(open(survey, 2)).rejects.toMatchObject({ code: 'invalid' });
  });
});

describe('isolamento entre tenants na aplicação', () => {
  it('recursos de outro tenant respondem como inexistentes', async () => {
    const { social, oneOnOne, survey } = setup();
    const admin = principal(ANA, TENANT_A, ['tenant-admin']);
    const s = await survey.create(admin, { title: 't', questions: [{ id: 'q', text: 't' }], segments: ['a'] });
    const m = await oneOnOne.schedule(principal(ANA), BRUNO, new Date());
    const post = await social.publishPost(principal(ANA, TENANT_A, ['manager']), 't', 'b');
    const outsider = (roles: string[]) => principal(ANA, TENANT_B, roles);
    await expect(survey.get(outsider([]), s.id)).rejects.toMatchObject({ code: 'not_found' });
    await expect(survey.results(outsider(['tenant-admin']), s.id)).rejects.toMatchObject({ code: 'not_found' });
    await expect(oneOnOne.context(outsider([]), m.id)).rejects.toMatchObject({ code: 'not_found' });
    await expect(social.confirmRead(outsider([]), post.id)).rejects.toMatchObject({ code: 'not_found' });
    expect(await social.feed(outsider([]))).toEqual([]);
    expect(await survey.list(outsider([]))).toEqual([]);
  });
});
