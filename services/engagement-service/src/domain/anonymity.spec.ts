import { randomUUID } from 'node:crypto';
import { aggregate, ROOT_SEGMENT } from './anonymity';
import { buildResponse, createSurvey, redactPii, type Survey, type SurveyResponse } from './survey';

const identity = <T>(a: T[]) => a;
const now = new Date('2026-11-21T12:00:00Z');

const survey = (segments: string[], k = 5): Survey =>
  createSurvey({ id: randomUUID(), tenantId: randomUUID(), title: 'Clima', questions: [{ id: 'q1', text: 'Estou bem' }], segments, minGroupSize: k, now });

const answers = (s: Survey, key: string, n: number, score = 4): SurveyResponse[] =>
  Array.from({ length: n }, () => buildResponse(s, { id: randomUUID(), segmentKey: key, scores: { q1: score }, now }));

const keys = (r: ReturnType<typeof aggregate>) => r.segments.map((s) => s.segmentKey);

describe('k-anonimato (ADR 0007)', () => {
  const s = survey(['obras', 'obras/sp', 'obras/rj', 'vendas']);

  it('segmento com k-1 respostas não aparece, mas conta no nível acima', () => {
    const r = aggregate(s, [...answers(s, 'obras/sp', 4), ...answers(s, 'obras/rj', 6)], identity);
    // sp (4) fica oculto e entra em obras (10); rj também sai, senão obras - rj revelaria os 4 de sp.
    expect(keys(r)).toEqual([ROOT_SEGMENT, 'obras']);
    expect(r.segments.find((x) => x.segmentKey === 'obras')?.respondents).toBe(10);
  });

  it('vários segmentos ocultos que somam k ou mais não obrigam a suprimir os vizinhos', () => {
    const wide = survey(['obras', 'obras/sp', 'obras/rj', 'obras/mg']);
    const r = aggregate(wide, [...answers(wide, 'obras/sp', 3), ...answers(wide, 'obras/mg', 3), ...answers(wide, 'obras/rj', 6)], identity);
    expect(keys(r)).toEqual([ROOT_SEGMENT, 'obras', 'obras/rj']);
  });

  it('segmento com exatamente k respostas aparece', () => {
    const r = aggregate(s, answers(s, 'vendas', 5), identity);
    expect(keys(r)).toContain('vendas');
  });

  it('nada é publicado quando o total é menor que k', () => {
    const r = aggregate(s, answers(s, 'vendas', 4), identity);
    expect(r.segments).toEqual([]);
    expect(r.suppressed).toBeGreaterThan(0);
  });

  it('supressão complementar: pai menos filho publicado não pode isolar menos de k pessoas', () => {
    // obras = 5 (sp) + 2 (rj, abaixo de k). Pai - sp = 2 pessoas: sp também precisa sair.
    const r = aggregate(s, [...answers(s, 'obras/sp', 5), ...answers(s, 'obras/rj', 2)], identity);
    expect(keys(r)).toEqual([ROOT_SEGMENT, 'obras']);
  });

  it('supressão complementar vale entre níveis: raiz menos áreas também respeita k', () => {
    const r = aggregate(s, [...answers(s, 'vendas', 6), ...answers(s, 'obras', 2)], identity);
    // raiz = 8, vendas = 6, resíduo = 2 (obras, abaixo de k): vendas sai, só a raiz fica.
    expect(keys(r)).toEqual([ROOT_SEGMENT]);
  });

  it('o resíduo de nenhuma pessoa não exige supressão', () => {
    const r = aggregate(s, [...answers(s, 'obras/sp', 5), ...answers(s, 'obras/rj', 5)], identity);
    expect(keys(r)).toEqual([ROOT_SEGMENT, 'obras', 'obras/rj', 'obras/sp']);
  });

  it('para qualquer combinação, nenhuma diferença entre cortes publicados isola menos de k pessoas', () => {
    for (let sp = 0; sp <= 8; sp++) {
      for (let rj = 0; rj <= 8; rj++) {
        for (let vendas = 0; vendas <= 8; vendas++) {
          const rs = [...answers(s, 'obras/sp', sp), ...answers(s, 'obras/rj', rj), ...answers(s, 'vendas', vendas)];
          const r = aggregate(s, rs, identity);
          const size = (key: string) => r.segments.find((x) => x.segmentKey === key)?.respondents;
          for (const parent of [ROOT_SEGMENT, 'obras']) {
            const p = size(parent);
            if (p === undefined) continue;
            const kids = r.segments.filter((x) => (parent === ROOT_SEGMENT ? x.segmentKey !== ROOT_SEGMENT && !x.segmentKey.includes('/') : x.segmentKey.startsWith('obras/')));
            const residual = p - kids.reduce((n, x) => n + x.respondents, 0);
            expect(residual === 0 || residual >= 5).toBe(true);
          }
          for (const seg of r.segments) expect(seg.respondents).toBeGreaterThanOrEqual(5);
        }
      }
    }
  });

  it('calcula a média por pergunta', () => {
    const r = aggregate(s, [...answers(s, 'vendas', 3, 2), ...answers(s, 'vendas', 2, 5)], identity);
    expect(r.segments.find((x) => x.segmentKey === 'vendas')?.scores.q1).toBe(3.2);
  });

  it('comentários só saem de segmentos publicados e já sem dados pessoais', () => {
    const rs = [
      ...Array.from({ length: 5 }, () => buildResponse(s, { id: randomUUID(), segmentKey: 'vendas', scores: { q1: 3 }, comment: 'falar com joao@example.com ou 11 98888-7777', now })),
    ];
    const r = aggregate(s, [...rs.slice(0, 5), ...answers(s, 'obras/sp', 1)].map((x, i) => (i === 5 ? { ...x, comment: 'segredo do sp' } : x)), identity);
    const small = aggregate(s, [{ ...answers(s, 'obras/sp', 1)[0]!, comment: 'segredo do sp' }], identity);
    expect(JSON.stringify(small)).not.toContain('segredo do sp');
    const all = JSON.stringify(r);
    expect(all).not.toContain('joao@example.com');
    expect(all).toContain('[removido]');
  });

  it('o resultado nunca contém identidade nem horário de envio', () => {
    const r = aggregate(s, answers(s, 'vendas', 6), identity);
    expect(JSON.stringify(r)).not.toMatch(/employee|submitted|created_at/i);
  });
});

describe('criação da pesquisa e resposta', () => {
  it('k não pode ficar abaixo de 3, mas pode subir', () => {
    expect(() => survey(['a'], 2)).toThrow(/menor que 3/);
    expect(survey(['a'], 8).minGroupSize).toBe(8);
    expect(survey(['a']).minGroupSize).toBe(5);
  });

  it('segmentos são definidos antes da coleta e a resposta fora deles é recusada', () => {
    const s = survey(['obras']);
    expect(() => buildResponse(s, { id: randomUUID(), segmentKey: 'rh', scores: { q1: 3 }, now })).toThrow(/segmento/);
  });

  it('a resposta não tem campo de identidade e a data não tem hora', () => {
    const s = survey(['obras']);
    const r = buildResponse(s, { id: randomUUID(), segmentKey: 'obras', scores: { q1: 3 }, now });
    expect(Object.keys(r).sort()).toEqual(['comment', 'id', 'scores', 'segmentKey', 'submittedOn', 'surveyId', 'tenantId']);
    expect(r.submittedOn).toBe('2026-11-21');
  });

  it('notas fora de 1-5, perguntas faltando e pesquisa encerrada são recusadas', () => {
    const s = survey(['obras']);
    expect(() => buildResponse(s, { id: 'x', segmentKey: 'obras', scores: { q1: 6 }, now })).toThrow();
    expect(() => buildResponse(s, { id: 'x', segmentKey: 'obras', scores: {}, now })).toThrow();
    expect(() => buildResponse({ ...s, status: 'closed' }, { id: 'x', segmentKey: 'obras', scores: { q1: 3 }, now })).toThrow(/encerrada/);
  });

  it('redactPii remove e-mail, CPF, telefone e menções', () => {
    expect(redactPii('a@b.com 123.456.789-09 (11) 91234-5678 @maria ok')).toBe('[removido] [removido] [removido] [removido] ok');
  });
});
