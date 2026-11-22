import { aggregate, shuffle, type SurveyAggregate } from '../domain/anonymity';
import { conflict, invalid, notFound } from '../domain/errors';
import { buildResponse, closeSurvey, createSurvey, type Survey, type SurveyQuestion } from '../domain/survey';
import type { Deps, Tx } from './ports';
import { requireRole, type Principal } from './principal';

export interface SurveyResults extends SurveyAggregate {
  surveyId: string;
  participation: number;
}

export class SurveyService {
  constructor(
    private readonly d: Deps,
    private readonly shuffler: <T>(a: T[]) => T[] = shuffle,
  ) {}

  async create(
    p: Principal,
    input: { title: string; questions: SurveyQuestion[]; segments: string[]; minGroupSize?: number },
  ): Promise<Survey> {
    requireRole(p, 'tenant-admin');
    return this.d.store.inTenant(p.tenantId, async (tx) => {
      const settings = await tx.settings.get();
      const s = createSurvey({
        ...input,
        minGroupSize: input.minGroupSize ?? settings?.minGroupSize,
        id: this.d.newId(),
        tenantId: p.tenantId,
        now: this.d.now(),
      });
      await tx.surveys.insert(s);
      return s;
    });
  }

  get(p: Principal, id: string): Promise<Survey & { alreadyAnswered: boolean }> {
    return this.d.store.inTenant(p.tenantId, async (tx) => ({
      ...(await this.load(tx, id)),
      alreadyAnswered: await tx.surveys.hasParticipated(id, p.employeeId),
    }));
  }

  /**
   * Participação e resposta são gravadas na mesma transação, mas em tabelas sem vínculo entre si.
   * O segmento vem do token, calculado agora, e a resposta não carrega employeeId.
   */
  async answer(p: Principal, surveyId: string, scores: Record<string, number>, comment?: string): Promise<void> {
    if (!p.segmentKey) throw invalid('o token não traz o segmento do colaborador');
    const segmentKey = p.segmentKey;
    return this.d.store.inTenant(p.tenantId, async (tx) => {
      const survey = await this.load(tx, surveyId);
      const response = buildResponse(survey, { id: this.d.newId(), segmentKey, scores, comment, now: this.d.now() });
      const first = await tx.surveys.addParticipation(surveyId, p.employeeId, response.submittedOn);
      if (!first) throw conflict('este colaborador já respondeu à pesquisa');
      await tx.surveys.insertResponse(response);
    });
  }

  async results(p: Principal, surveyId: string): Promise<SurveyResults> {
    requireRole(p, 'tenant-admin', 'manager');
    return this.d.store.inTenant(p.tenantId, (tx) => this.compute(tx, surveyId));
  }

  /** Encerra e publica PesquisaClimaEncerrada só com agregados que atingiram k (sem comentários). */
  async close(p: Principal, surveyId: string): Promise<Survey> {
    requireRole(p, 'tenant-admin');
    return this.d.store.inTenant(p.tenantId, async (tx) => {
      const closed = closeSurvey(await this.load(tx, surveyId));
      await tx.surveys.save(closed);
      const res = await this.compute(tx, surveyId);
      await tx.outbox.add({
        id: this.d.newId(),
        tenantId: p.tenantId,
        aggregateType: 'survey',
        aggregateId: surveyId,
        eventType: 'PesquisaClimaEncerrada',
        eventVersion: 1,
        payload: {
          survey_id: surveyId,
          min_group_size: closed.minGroupSize,
          segments: res.segments.map((s) => ({ segment_key: s.segmentKey, respondents: s.respondents, scores: s.scores })),
        },
        occurredAt: this.d.now(),
      });
      return closed;
    });
  }

  private async compute(tx: Tx, surveyId: string): Promise<SurveyResults> {
    const survey = await this.load(tx, surveyId);
    const [responses, participation] = await Promise.all([tx.surveys.listResponses(surveyId), tx.surveys.countParticipation(surveyId)]);
    return { surveyId, participation, ...aggregate(survey, responses, this.shuffler) };
  }

  private async load(tx: Tx, id: string): Promise<Survey> {
    const s = await tx.surveys.get(id);
    if (!s) throw notFound('pesquisa não encontrada');
    return s;
  }
}
