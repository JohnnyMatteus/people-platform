import { conflict, invalid } from './errors';

export const DEFAULT_MIN_GROUP_SIZE = 5;
/** ADR 0007: o tenant só pode aumentar o k, nunca ficar abaixo de 3. */
export const FLOOR_MIN_GROUP_SIZE = 3;

export interface SurveyQuestion {
  id: string;
  text: string;
}

export interface Survey {
  id: string;
  tenantId: string;
  title: string;
  status: 'open' | 'closed';
  questions: SurveyQuestion[];
  segments: string[];
  minGroupSize: number;
  createdAt: Date;
}

/**
 * Resposta de clima. Não tem employee_id, nem horário (só a data): nada aqui liga a resposta a uma
 * pessoa (ADR 0007). Qualquer campo novo precisa passar pelo teste de anonimato.
 */
export interface SurveyResponse {
  id: string;
  tenantId: string;
  surveyId: string;
  segmentKey: string;
  scores: Record<string, number>;
  comment: string | null;
  submittedOn: string;
}

const SEGMENT = /^[a-z0-9-]+(\/[a-z0-9-]+)?$/;

export function createSurvey(input: {
  id: string;
  tenantId: string;
  title: string;
  questions: SurveyQuestion[];
  segments: string[];
  minGroupSize?: number;
  now: Date;
}): Survey {
  const k = input.minGroupSize ?? DEFAULT_MIN_GROUP_SIZE;
  if (!Number.isInteger(k) || k < FLOOR_MIN_GROUP_SIZE) {
    throw invalid(`tamanho mínimo de grupo não pode ser menor que ${FLOOR_MIN_GROUP_SIZE}`);
  }
  if (!input.title.trim()) throw invalid('pesquisa sem título');
  if (input.questions.length === 0) throw invalid('pesquisa sem perguntas');
  if (new Set(input.questions.map((q) => q.id)).size !== input.questions.length) {
    throw invalid('ids de pergunta repetidos');
  }
  const segments = [...new Set(input.segments)];
  if (segments.length === 0) throw invalid('defina os segmentos antes da coleta');
  const bad = segments.find((s) => !SEGMENT.test(s));
  if (bad) throw invalid(`segmento inválido: ${bad} (use "area" ou "area/unidade")`);
  return {
    id: input.id,
    tenantId: input.tenantId,
    title: input.title.trim(),
    status: 'open',
    questions: input.questions,
    segments,
    minGroupSize: k,
    createdAt: input.now,
  };
}

export function closeSurvey(s: Survey): Survey {
  if (s.status === 'closed') throw conflict('pesquisa já encerrada');
  return { ...s, status: 'closed' };
}

const EMAIL = /[\w.+-]+@[\w-]+(\.[\w-]+)+/g;
const CPF = /\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b/g;
const PHONE = /(\+?\d{2}\s?)?\(?\d{2}\)?\s?9?\d{4}[-\s]?\d{4}\b/g;
const HANDLE = /@\w+/g;

/** Remove e-mails, telefones, CPFs e @menções do texto livre antes de gravar. */
export function redactPii(text: string): string {
  return text
    .replace(EMAIL, '[removido]')
    .replace(CPF, '[removido]')
    .replace(PHONE, '[removido]')
    .replace(HANDLE, '[removido]')
    .trim();
}

export function buildResponse(
  survey: Survey,
  input: { id: string; segmentKey: string; scores: Record<string, number>; comment?: string; now: Date },
): SurveyResponse {
  if (survey.status !== 'open') throw conflict('pesquisa encerrada');
  if (!survey.segments.includes(input.segmentKey)) throw invalid('segmento não permitido nesta pesquisa');
  const ids = survey.questions.map((q) => q.id);
  const given = Object.keys(input.scores);
  if (given.length !== ids.length || !ids.every((id) => given.includes(id))) {
    throw invalid('responda todas as perguntas, e só elas');
  }
  for (const v of Object.values(input.scores)) {
    if (!Number.isInteger(v) || v < 1 || v > 5) throw invalid('notas vão de 1 a 5');
  }
  const comment = input.comment ? redactPii(input.comment) : '';
  return {
    id: input.id,
    tenantId: survey.tenantId,
    surveyId: survey.id,
    segmentKey: input.segmentKey,
    scores: input.scores,
    comment: comment || null,
    submittedOn: input.now.toISOString().slice(0, 10),
  };
}
