import { randomUUID } from 'node:crypto';
import { GamificationService } from '../src/app/gamification-service';
import { OneOnOneService } from '../src/app/one-on-one-service';
import { PrivacyService } from '../src/app/privacy-service';
import { SocialService } from '../src/app/social-service';
import { SurveyService } from '../src/app/survey-service';
import type { ErasureRequestedEnvelope } from '../src/domain/privacy';
import { ANA, BRUNO, CARLA, depsFor, principal, TENANT_A, TENANT_B } from './support/fixtures';
import { MemoryStore } from './support/memory-store';

const erasure = (tenant: string, subject: string, eventId = randomUUID()): ErasureRequestedEnvelope => ({
  event_id: eventId,
  event_type: 'TitularExclusaoSolicitada',
  event_version: 1,
  tenant_id: tenant,
  data: { request_id: randomUUID(), subject_id: subject },
});

/** A mesma pessoa (mesmo id) com dados nos dois tenants, e o Bruno e a Carla só no A. */
async function seed(store: MemoryStore) {
  const d = depsFor(store);
  const social = new SocialService(d);
  const gamification = new GamificationService(d);
  const admin = principal(randomUUID(), TENANT_A, ['tenant-admin', 'manager'], 'obras');
  for (const tenant of [TENANT_A, TENANT_B]) {
    const ana = principal(ANA, tenant, ['manager', 'employee'], 'obras');
    await social.giveFeedback(ana, BRUNO, 'bom trabalho');
    await social.giveFeedback(principal(BRUNO, tenant), ANA, 'obrigado');
    await social.praise(ana, [BRUNO, CARLA], 'parabéns aos dois', []);
    await social.praise(principal(CARLA, tenant), [ANA], 'valeu', []);
    await new OneOnOneService(d).schedule(ana, BRUNO, new Date('2026-12-01T10:00:00Z'));
    const post = await social.publishPost(ana, 'Aviso da Ana', 'texto');
    await social.confirmRead(principal(BRUNO, tenant), post.id);
    // A Ana também lê um comunicado de outra pessoa: sem isso ela não teria nenhuma confirmação de leitura para apagar.
    const bruno = principal(BRUNO, tenant, ['manager']);
    await social.confirmRead(ana, (await social.publishPost(bruno, 'Aviso do Bruno', 'texto')).id);
  }
  for (const e of store.outbox.filter((o) => o.eventType === 'ElogioPublicado')) {
    await gamification.onPraisePublished({ event_id: e.id, event_type: 'ElogioPublicado', tenant_id: e.tenantId, data: e.payload as never });
  }
  const survey = new SurveyService(d, (a) => a);
  const s = await survey.create(admin, { title: 'Clima', questions: [{ id: 'q1', text: 'ok' }], segments: ['obras'] });
  await survey.answer(principal(ANA, TENANT_A, ['employee'], 'obras'), s.id, { q1: 4 }, 'comentário anônimo');
}

const countFor = (store: MemoryStore, tenant: string, id: string) => ({
  feedbacks: store.feedbacks.filter((f) => f.tenantId === tenant && (f.fromId === id || f.toId === id)).length,
  praises: store.praises.filter((p) => p.tenantId === tenant && (p.fromId === id || p.toIds.includes(id))).length,
  meetings: store.meetings.filter((m) => m.tenantId === tenant && (m.leaderId === id || m.participantId === id)).length,
  posts: store.posts.filter((p) => p.tenantId === tenant && p.authorId === id).length,
  receipts: store.receipts.filter((r) => r.tenantId === tenant && r.employeeId === id).length,
  ledger: store.ledger.filter((l) => l.tenantId === tenant && l.employeeId === id).length,
  participation: store.participation.filter((p) => p.tenantId === tenant && p.employeeId === id).length,
});

describe('exclusão de titular (LGPD)', () => {
  it('apaga tudo da pessoa no tenant do evento, preserva os outros e o mesmo id em outro tenant', async () => {
    const store = new MemoryStore();
    await seed(store);
    const before = countFor(store, TENANT_A, ANA);
    expect(Object.values(before).every((n) => n > 0)).toBe(true);
    const bruno = countFor(store, TENANT_A, BRUNO);
    const anaInB = countFor(store, TENANT_B, ANA);

    const applied = await new PrivacyService(depsFor(store)).onErasureRequested(erasure(TENANT_A, ANA));

    expect(applied).toBe(true);
    expect(countFor(store, TENANT_A, ANA)).toEqual({ feedbacks: 0, praises: 0, meetings: 0, posts: 0, receipts: 0, ledger: 0, participation: 0 });
    expect(countFor(store, TENANT_B, ANA)).toEqual(anaInB);
    // O elogio dado ao Bruno e à Carla juntos some (foi a Ana quem deu); o valeu da Carla para a Ana some (só ela recebia).
    expect(store.praises.filter((p) => p.tenantId === TENANT_A)).toEqual([]);
    expect(countFor(store, TENANT_A, BRUNO).ledger).toBeLessThanOrEqual(bruno.ledger);
  });

  it('elogio recebido junto com outras pessoas só perde o nome do titular', async () => {
    const store = new MemoryStore();
    const social = new SocialService(depsFor(store));
    await social.praise(principal(ANA), [BRUNO, CARLA], 'ótimo time', []);

    await new PrivacyService(depsFor(store)).onErasureRequested(erasure(TENANT_A, CARLA));

    expect(store.praises).toHaveLength(1);
    expect(store.praises[0]!.toIds).toEqual([BRUNO]);
  });

  it('reentrega do mesmo evento não tem efeito e uma pesquisa respondida continua anônima', async () => {
    const store = new MemoryStore();
    await seed(store);
    const service = new PrivacyService(depsFor(store));
    const event = erasure(TENANT_A, ANA);

    expect(await service.onErasureRequested(event)).toBe(true);
    const snapshot = JSON.stringify(store.feedbacks);
    expect(await service.onErasureRequested(event)).toBe(false);
    expect(JSON.stringify(store.feedbacks)).toBe(snapshot);
    // As respostas de clima nunca tiveram identidade (ADR 0007): permanecem, e só a participação some.
    expect(store.responses).toHaveLength(1);
  });
});

describe('lápide: o dado que chega depois do pedido não ressuscita o titular', () => {
  it('pontos de elogio e de leitura são ignorados para quem já foi apagado, e só para ele', async () => {
    const store = new MemoryStore();
    const d = depsFor(store);
    const social = new SocialService(d);
    const gamification = new GamificationService(d);

    // A exclusão da Carla chega antes de qualquer dado dela.
    await new PrivacyService(d).onErasureRequested(erasure(TENANT_A, CARLA));
    await social.praise(principal(ANA), [BRUNO, CARLA], 'parabéns aos dois', []);
    for (const e of store.outbox.filter((o) => o.eventType === 'ElogioPublicado')) {
      await gamification.onPraisePublished({ event_id: e.id, event_type: 'ElogioPublicado', tenant_id: e.tenantId, data: e.payload as never });
    }
    const post = await social.publishPost(principal(ANA, TENANT_A, ['manager']), 'Aviso', 'texto');
    await social.confirmRead(principal(CARLA), post.id);

    expect(store.ledger.filter((l) => l.employeeId === CARLA)).toEqual([]);
    expect(store.badges.filter((b) => b.employeeId === CARLA)).toEqual([]);
    expect(store.ledger.filter((l) => l.employeeId === BRUNO).length).toBeGreaterThan(0);
    // A lápide vale só no tenant do pedido: a mesma pessoa no tenant B continua pontuando.
    await social.praise(principal(ANA, TENANT_B), [CARLA], 'oi', []);
    for (const e of store.outbox.filter((o) => o.eventType === 'ElogioPublicado' && o.tenantId === TENANT_B)) {
      await gamification.onPraisePublished({ event_id: e.id, event_type: 'ElogioPublicado', tenant_id: e.tenantId, data: e.payload as never });
    }
    expect(store.ledger.filter((l) => l.tenantId === TENANT_B && l.employeeId === CARLA).length).toBeGreaterThan(0);
  });
});
