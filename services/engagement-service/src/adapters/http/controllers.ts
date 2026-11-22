import { Body, Controller, Get, HttpCode, Inject, Param, ParseUUIDPipe, Post, Query, UseGuards } from '@nestjs/common';
import { z } from 'zod';
import type { GamificationService } from '../../app/gamification-service';
import type { OneOnOneService } from '../../app/one-on-one-service';
import type { Principal } from '../../app/principal';
import type { SocialService } from '../../app/social-service';
import type { SurveyService } from '../../app/survey-service';
import type { OneOnOne } from '../../domain/one-on-one';
import type { Feedback, Praise } from '../../domain/social';
import type { Survey } from '../../domain/survey';
import { AuthGuard, Me, parse } from './support';

const uuid = z.string().uuid();
const GiveFeedback = z.object({ to_employee_id: uuid, body: z.string().min(1).max(4000) }).strict();
const GivePraise = z
  .object({ to_employee_ids: z.array(uuid).min(1).max(10), message: z.string().min(1).max(1000), values: z.array(z.string()).max(5).default([]) })
  .strict();
const Schedule = z.object({ participant_id: uuid, scheduled_for: z.string().datetime() }).strict();
const AgendaItem = z.object({ text: z.string().min(1).max(500) }).strict();
const CreateSurvey = z
  .object({
    title: z.string().min(1),
    questions: z.array(z.object({ id: z.string().min(1), text: z.string().min(1) }).strict()).min(1),
    segments: z.array(z.string()).min(1),
    min_group_size: z.number().int().optional(),
  })
  .strict();
const Answer = z.object({ scores: z.record(z.string(), z.number()), comment: z.string().max(2000).optional() }).strict();
const PublishPost = z.object({ title: z.string().min(1).max(200), body: z.string().min(1).max(8000) }).strict();

const feedbackOut = (f: Feedback) => ({
  id: f.id, from_employee_id: f.fromId, to_employee_id: f.toId, body: f.body, created_at: f.createdAt,
});
const praiseOut = (p: Praise) => ({
  id: p.id, from_employee_id: p.fromId, to_employee_ids: p.toIds, message: p.message, values: p.values, created_at: p.createdAt,
});
const agendaOut = (i: OneOnOne['agenda'][number]) => ({ id: i.id, text: i.text, added_by: i.addedBy, done: i.done });
const meetingOut = (m: OneOnOne) => ({
  id: m.id, leader_id: m.leaderId, participant_id: m.participantId, scheduled_for: m.scheduledFor, agenda: m.agenda.map(agendaOut),
});
const surveyOut = (s: Survey, alreadyAnswered?: boolean) => ({
  id: s.id, title: s.title, status: s.status, questions: s.questions, segments: s.segments, min_group_size: s.minGroupSize,
  ...(alreadyAnswered === undefined ? {} : { already_answered: alreadyAnswered }),
});

export const SERVICES = {
  social: 'SocialService',
  oneOnOne: 'OneOnOneService',
  survey: 'SurveyService',
  gamification: 'GamificationService',
} as const;

@Controller('v1')
@UseGuards(AuthGuard)
export class SocialController {
  constructor(@Inject(SERVICES.social) private readonly s: SocialService) {}

  @Post('feedbacks')
  async giveFeedback(@Me() me: Principal, @Body() body: unknown) {
    const b = parse(GiveFeedback, body);
    return feedbackOut(await this.s.giveFeedback(me, b.to_employee_id, b.body));
  }

  @Get('feedbacks')
  async listFeedbacks(@Me() me: Principal, @Query('with') withId?: string) {
    return (await this.s.listFeedbacks(me, withId ? parse(uuid, withId) : undefined)).map(feedbackOut);
  }

  @Post('praises')
  async praise(@Me() me: Principal, @Body() body: unknown) {
    const b = parse(GivePraise, body);
    return praiseOut(await this.s.praise(me, b.to_employee_ids, b.message, b.values));
  }

  @Get('praises')
  async feed(@Me() me: Principal) {
    return (await this.s.feed(me)).map(praiseOut);
  }

  @Post('posts')
  async publishPost(@Me() me: Principal, @Body() body: unknown) {
    const b = parse(PublishPost, body);
    const p = await this.s.publishPost(me, b.title, b.body);
    return { id: p.id, author_id: p.authorId, title: p.title, body: p.body, created_at: p.createdAt, read_count: 0, read_by_me: false };
  }

  @Get('posts')
  async listPosts(@Me() me: Principal) {
    return (await this.s.listPosts(me)).map((p) => ({
      id: p.id, author_id: p.authorId, title: p.title, body: p.body, created_at: p.createdAt, read_count: p.readCount, read_by_me: p.readByMe,
    }));
  }

  @Post('posts/:postId/read-receipts')
  @HttpCode(204)
  async confirmRead(@Me() me: Principal, @Param('postId', ParseUUIDPipe) postId: string) {
    await this.s.confirmRead(me, postId);
  }

  @Get('posts/:postId/read-receipts')
  async receipts(@Me() me: Principal, @Param('postId', ParseUUIDPipe) postId: string) {
    return (await this.s.receipts(me, postId)).map((r) => ({ employee_id: r.employeeId, read_at: r.readAt }));
  }
}

@Controller('v1/one-on-ones')
@UseGuards(AuthGuard)
export class OneOnOneController {
  constructor(@Inject(SERVICES.oneOnOne) private readonly s: OneOnOneService) {}

  @Post()
  async schedule(@Me() me: Principal, @Body() body: unknown) {
    const b = parse(Schedule, body);
    return meetingOut(await this.s.schedule(me, b.participant_id, new Date(b.scheduled_for)));
  }

  @Get()
  async list(@Me() me: Principal) {
    return (await this.s.list(me)).map(meetingOut);
  }

  @Post(':meetingId/agenda-items')
  async addItem(@Me() me: Principal, @Param('meetingId', ParseUUIDPipe) id: string, @Body() body: unknown) {
    return meetingOut(await this.s.addItem(me, id, parse(AgendaItem, body).text));
  }

  @Post(':meetingId/agenda-items/:itemId/done')
  @HttpCode(200)
  async done(@Me() me: Principal, @Param('meetingId', ParseUUIDPipe) id: string, @Param('itemId', ParseUUIDPipe) item: string) {
    return meetingOut(await this.s.completeItem(me, id, item));
  }

  @Get(':meetingId/context')
  async context(@Me() me: Principal, @Param('meetingId', ParseUUIDPipe) id: string) {
    const c = await this.s.context(me, id);
    return {
      meeting: meetingOut(c.meeting),
      recent_feedbacks: c.recentFeedbacks.map(feedbackOut),
      recent_praises: c.recentPraises.map(praiseOut),
      open_agenda: c.openAgenda.map(agendaOut),
    };
  }
}

@Controller('v1/surveys')
@UseGuards(AuthGuard)
export class SurveyController {
  constructor(@Inject(SERVICES.survey) private readonly s: SurveyService) {}

  @Post()
  async create(@Me() me: Principal, @Body() body: unknown) {
    const b = parse(CreateSurvey, body);
    return surveyOut(await this.s.create(me, { ...b, minGroupSize: b.min_group_size }));
  }

  @Get()
  async list(@Me() me: Principal) {
    return (await this.s.list(me)).map(({ alreadyAnswered, ...s }) => surveyOut(s, alreadyAnswered));
  }

  @Get(':surveyId')
  async get(@Me() me: Principal, @Param('surveyId', ParseUUIDPipe) id: string) {
    const { alreadyAnswered, ...s } = await this.s.get(me, id);
    return surveyOut(s, alreadyAnswered);
  }

  @Post(':surveyId/responses')
  @HttpCode(202)
  async answer(@Me() me: Principal, @Param('surveyId', ParseUUIDPipe) id: string, @Body() body: unknown) {
    const b = parse(Answer, body);
    await this.s.answer(me, id, b.scores, b.comment);
  }

  @Post(':surveyId/close')
  @HttpCode(200)
  async close(@Me() me: Principal, @Param('surveyId', ParseUUIDPipe) id: string) {
    return surveyOut(await this.s.close(me, id));
  }

  @Get(':surveyId/results')
  async results(@Me() me: Principal, @Param('surveyId', ParseUUIDPipe) id: string) {
    const r = await this.s.results(me, id);
    return {
      survey_id: r.surveyId,
      min_group_size: r.minGroupSize,
      participation: r.participation,
      suppressed: r.suppressed,
      segments: r.segments.map((x) => ({ segment_key: x.segmentKey, respondents: x.respondents, scores: x.scores, comments: x.comments })),
    };
  }
}

@Controller('v1')
@UseGuards(AuthGuard)
export class GamificationController {
  constructor(@Inject(SERVICES.gamification) private readonly s: GamificationService) {}

  @Get('points/me')
  async mine(@Me() me: Principal) {
    const r = await this.s.mine(me);
    return {
      balance: r.balance,
      ledger: r.ledger.map((l) => ({ points: l.points, reason: l.reason, created_at: l.createdAt })),
      badges: r.badges.map((b) => ({ code: b.code, awarded_at: b.awardedAt })),
    };
  }

  @Get('leaderboard')
  async leaderboard(@Me() me: Principal) {
    return (await this.s.leaderboard(me)).map((e) => ({ employee_id: e.employeeId, points: e.points }));
  }
}
