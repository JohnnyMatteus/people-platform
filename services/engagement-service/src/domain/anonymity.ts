import type { Survey, SurveyResponse } from './survey';

export const ROOT_SEGMENT = '_all';

export interface SegmentResult {
  segmentKey: string;
  respondents: number;
  scores: Record<string, number>;
  comments: string[];
}

export interface SurveyAggregate {
  minGroupSize: number;
  segments: SegmentResult[];
  /** Quantos cortes foram omitidos; não diz quais. */
  suppressed: number;
}

const parentOf = (key: string): string => (key === ROOT_SEGMENT ? '' : key.includes('/') ? key.split('/')[0]! : ROOT_SEGMENT);

const inside = (responseKey: string, node: string) =>
  node === ROOT_SEGMENT || responseKey === node || responseKey.startsWith(`${node}/`);

/**
 * Agrega as respostas respeitando k-anonimato (ADR 0007):
 *  - só cortes pré-definidos (raiz, áreas e unidades); nunca filtros ad hoc;
 *  - corte com menos de k respondentes não sai: suas respostas ficam apenas no nível acima;
 *  - supressão complementar: se (pai menos filhos publicados) deixaria um resíduo de 1 a k-1
 *    pessoas, o menor filho publicado também é suprimido, até o resíduo ser 0 ou >= k.
 */
export function aggregate(survey: Survey, responses: SurveyResponse[], shuffle: <T>(a: T[]) => T[]): SurveyAggregate {
  const k = survey.minGroupSize;
  const nodes = new Set<string>([ROOT_SEGMENT]);
  for (const s of survey.segments) {
    nodes.add(s);
    nodes.add(parentOf(s));
  }
  const count = (node: string) => responses.filter((r) => inside(r.segmentKey, node)).length;
  const size = new Map([...nodes].map((n) => [n, count(n)]));
  const childrenOf = (node: string) => [...nodes].filter((n) => n !== ROOT_SEGMENT && parentOf(n) === node);

  const published = new Set<string>();
  const visit = (node: string) => {
    if ((size.get(node) ?? 0) < k) return; // corte abaixo de k e todos os seus descendentes ficam de fora
    published.add(node);
    const kids = childrenOf(node).filter((c) => (size.get(c) ?? 0) >= k);
    const visible = new Set(kids);
    const residual = () => (size.get(node) ?? 0) - [...visible].reduce((sum, c) => sum + (size.get(c) ?? 0), 0);
    while (visible.size > 0 && residual() > 0 && residual() < k) {
      const smallest = [...visible].sort((a, b) => (size.get(a)! - size.get(b)!) || a.localeCompare(b))[0]!;
      visible.delete(smallest);
    }
    visible.forEach(visit);
  };
  visit(ROOT_SEGMENT);

  const segments = [...published].sort().map((node): SegmentResult => {
    const rs = responses.filter((r) => inside(r.segmentKey, node));
    const scores: Record<string, number> = {};
    for (const q of survey.questions) {
      const total = rs.reduce((sum, r) => sum + (r.scores[q.id] ?? 0), 0);
      scores[q.id] = Math.round((total / rs.length) * 100) / 100;
    }
    const comments = shuffle(rs.flatMap((r) => (r.comment ? [r.comment] : [])));
    return { segmentKey: node, respondents: rs.length, scores, comments };
  });
  return { minGroupSize: k, segments, suppressed: nodes.size - published.size };
}

/** Fisher-Yates; a ordem dos comentários não pode revelar a ordem de envio. */
export function shuffle<T>(items: T[], random: () => number = Math.random): T[] {
  const a = [...items];
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(random() * (i + 1));
    [a[i], a[j]] = [a[j]!, a[i]!];
  }
  return a;
}
