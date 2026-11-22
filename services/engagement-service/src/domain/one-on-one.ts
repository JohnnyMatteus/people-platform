import { invalid, notFound } from './errors';

export interface AgendaItem {
  id: string;
  text: string;
  addedBy: string;
  done: boolean;
}

export interface OneOnOne {
  id: string;
  tenantId: string;
  leaderId: string;
  participantId: string;
  scheduledFor: Date;
  agenda: AgendaItem[];
}

export function scheduleOneOnOne(
  id: string,
  tenantId: string,
  leaderId: string,
  participantId: string,
  scheduledFor: Date,
): OneOnOne {
  if (leaderId === participantId) throw invalid('1:1 precisa de duas pessoas');
  return { id, tenantId, leaderId, participantId, scheduledFor, agenda: [] };
}

export const isParticipant = (m: OneOnOne, employeeId: string) =>
  m.leaderId === employeeId || m.participantId === employeeId;

export function assertParticipant(m: OneOnOne, employeeId: string): void {
  // 404 e não 403: quem não participa não deve saber que a 1:1 existe.
  if (!isParticipant(m, employeeId)) throw notFound('1:1 não encontrada');
}

export function addAgendaItem(m: OneOnOne, itemId: string, employeeId: string, text: string): OneOnOne {
  assertParticipant(m, employeeId);
  if (!text.trim()) throw invalid('item de pauta vazio');
  return { ...m, agenda: [...m.agenda, { id: itemId, text: text.trim(), addedBy: employeeId, done: false }] };
}

export function completeAgendaItem(m: OneOnOne, itemId: string, employeeId: string): OneOnOne {
  assertParticipant(m, employeeId);
  if (!m.agenda.some((i) => i.id === itemId)) throw notFound('item de pauta não encontrado');
  return { ...m, agenda: m.agenda.map((i) => (i.id === itemId ? { ...i, done: true } : i)) };
}
