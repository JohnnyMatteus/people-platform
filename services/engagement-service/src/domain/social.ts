import { invalid } from './errors';

export interface Feedback {
  id: string;
  tenantId: string;
  fromId: string;
  toId: string;
  body: string;
  createdAt: Date;
}

export interface Praise {
  id: string;
  tenantId: string;
  fromId: string;
  toIds: string[];
  message: string;
  values: string[];
  createdAt: Date;
}

export interface Post {
  id: string;
  tenantId: string;
  authorId: string;
  title: string;
  body: string;
  createdAt: Date;
}

export interface ReadReceipt {
  employeeId: string;
  readAt: Date;
}

export function newFeedback(id: string, tenantId: string, fromId: string, toId: string, body: string, now: Date): Feedback {
  if (fromId === toId) throw invalid('feedback para si mesmo não é permitido');
  if (!body.trim()) throw invalid('feedback sem texto');
  return { id, tenantId, fromId, toId, body: body.trim(), createdAt: now };
}

export function newPraise(
  id: string,
  tenantId: string,
  fromId: string,
  toIds: string[],
  message: string,
  values: string[],
  now: Date,
): Praise {
  const unique = [...new Set(toIds)];
  if (unique.length === 0) throw invalid('elogio sem destinatário');
  if (unique.includes(fromId)) throw invalid('não é possível elogiar a si mesmo');
  if (!message.trim()) throw invalid('elogio sem texto');
  return { id, tenantId, fromId, toIds: unique, message: message.trim(), values: [...new Set(values)], createdAt: now };
}

/** Feedback é privado: só as duas partes enxergam. */
export const canSeeFeedback = (f: Feedback, employeeId: string) => f.fromId === employeeId || f.toId === employeeId;
