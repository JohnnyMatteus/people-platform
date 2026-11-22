import type { OutboxEvent } from '../domain/events';
import type { PointEntry, PointReason } from '../domain/gamification';
import type { OneOnOne } from '../domain/one-on-one';
import type { Feedback, Post, Praise, ReadReceipt } from '../domain/social';
import type { Survey, SurveyResponse } from '../domain/survey';
import type { TenantSettings } from '../domain/tenant-settings';

export interface FeedbackRepository {
  insert(f: Feedback): Promise<void>;
  /** Feedbacks em que o colaborador é autor ou destinatário, do mais recente ao mais antigo. */
  listFor(employeeId: string, withId?: string): Promise<Feedback[]>;
}

export interface PraiseRepository {
  insert(p: Praise): Promise<void>;
  recent(limit: number): Promise<Praise[]>;
  recentBetween(a: string, b: string, limit: number): Promise<Praise[]>;
}

export interface OneOnOneRepository {
  insert(m: OneOnOne): Promise<void>;
  get(id: string): Promise<OneOnOne | null>;
  save(m: OneOnOne): Promise<void>;
  listFor(employeeId: string): Promise<OneOnOne[]>;
}

export interface SurveyRepository {
  insert(s: Survey): Promise<void>;
  get(id: string): Promise<Survey | null>;
  save(s: Survey): Promise<void>;
  /** false se o colaborador já participou. Só guarda quem participou, nunca o que respondeu. */
  addParticipation(surveyId: string, employeeId: string, on: string): Promise<boolean>;
  hasParticipated(surveyId: string, employeeId: string): Promise<boolean>;
  countParticipation(surveyId: string): Promise<number>;
  insertResponse(r: SurveyResponse): Promise<void>;
  listResponses(surveyId: string): Promise<SurveyResponse[]>;
}

export interface PointsRepository {
  /** Idempotente por (evento de origem, pessoa, motivo). Devolve só as entradas realmente novas. */
  award(entries: PointEntry[], sourceEventId: string, now: Date, newId: () => string): Promise<PointEntry[]>;
  stats(employeeId: string): Promise<Record<PointReason, number>>;
  balance(employeeId: string): Promise<number>;
  ledger(employeeId: string, limit: number): Promise<{ points: number; reason: string; createdAt: Date }[]>;
  badges(employeeId: string): Promise<{ code: string; awardedAt: Date }[]>;
  addBadge(employeeId: string, code: string, now: Date): Promise<void>;
  leaderboard(limit: number): Promise<{ employeeId: string; points: number }[]>;
}

export interface PostRepository {
  insert(p: Post): Promise<void>;
  get(id: string): Promise<Post | null>;
  list(limit: number): Promise<Post[]>;
  /** true se a leitura é nova. */
  confirmRead(postId: string, employeeId: string, now: Date): Promise<boolean>;
  receipts(postId: string): Promise<ReadReceipt[]>;
  readCounts(postIds: string[]): Promise<Map<string, number>>;
  readBy(employeeId: string, postIds: string[]): Promise<Set<string>>;
}

export interface TenantSettingsRepository {
  get(): Promise<TenantSettings | null>;
  upsert(s: TenantSettings, now: Date): Promise<void>;
}

export interface ProcessedEventRepository {
  /** false se o evento já tinha sido consumido (deduplicação por event_id). */
  claim(eventId: string, eventType: string, now: Date): Promise<boolean>;
}

export interface OutboxRepository {
  add(e: OutboxEvent): Promise<void>;
}

/** Repositórios ligados a uma transação com o escopo de um tenant. */
export interface Tx {
  feedbacks: FeedbackRepository;
  praises: PraiseRepository;
  oneOnOnes: OneOnOneRepository;
  surveys: SurveyRepository;
  points: PointsRepository;
  posts: PostRepository;
  outbox: OutboxRepository;
  settings: TenantSettingsRepository;
  processedEvents: ProcessedEventRepository;
}

export interface Store {
  /** Executa fn numa transação em que só as linhas do tenant são visíveis (RLS). */
  inTenant<T>(tenantId: string, fn: (tx: Tx) => Promise<T>): Promise<T>;
}

export interface Deps {
  store: Store;
  now: () => Date;
  newId: () => string;
}
