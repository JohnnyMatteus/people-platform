export type PointReason = 'praise_received' | 'praise_given' | 'read_confirmed';

export const POINTS: Record<PointReason, number> = {
  praise_received: 10,
  praise_given: 5,
  read_confirmed: 1,
};

export interface PointEntry {
  employeeId: string;
  points: number;
  reason: PointReason;
}

/** Pontos gerados por um elogio publicado: quem recebe ganha mais que quem elogia. */
export function pointsForPraise(fromId: string, toIds: string[]): PointEntry[] {
  return [
    { employeeId: fromId, points: POINTS.praise_given, reason: 'praise_given' },
    ...toIds.map((id): PointEntry => ({ employeeId: id, points: POINTS.praise_received, reason: 'praise_received' })),
  ];
}

export interface BadgeRule {
  code: string;
  earned: (stats: Record<PointReason, number>) => boolean;
}

/** stats[reason] = quantas vezes a pessoa pontuou por esse motivo. */
export const BADGE_RULES: BadgeRule[] = [
  { code: 'primeiro-reconhecimento', earned: (s) => s.praise_received >= 1 },
  { code: 'reconhecido', earned: (s) => s.praise_received >= 10 },
  { code: 'generoso', earned: (s) => s.praise_given >= 10 },
  { code: 'leitor-assiduo', earned: (s) => s.read_confirmed >= 20 },
];

export function badgesEarned(stats: Record<PointReason, number>, alreadyHas: string[]): string[] {
  return BADGE_RULES.filter((r) => r.earned(stats) && !alreadyHas.includes(r.code)).map((r) => r.code);
}
