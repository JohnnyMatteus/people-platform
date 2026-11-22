import { describe, expect, it } from 'vitest'
import * as analytics from './analytics'
import * as engagement from './engagement'
import * as recruitment from './recruitment'

// Amostras no formato exato que cada serviço devolve (Presenter do PHP, DTOs do NestJS e modelos do FastAPI).
const id = (n: number) => `0197a000-0000-7000-8000-${String(n).padStart(12, '0')}`

describe('schemas x respostas dos serviços', () => {
  it('recrutamento: vaga, candidatura com relações não carregadas (null) e página por cursor', () => {
    const job = { id: id(1), title: 'Eletricista', unit: '', status: 'open', stages: ['triagem', 'contratado'], created_at: '2026-11-22T10:00:00+00:00' }
    expect(recruitment.jobPage.parse({ items: [job], next_cursor: null }).items).toHaveLength(1)
    const app = { id: id(2), job_id: id(1), stage: 'triagem', status: 'active', start_date: null, candidate: { id: id(3), name: 'Ana', email: 'ana@example.com' }, interviews: null }
    expect(recruitment.applicationPage.parse({ items: [app], next_cursor: 'abc' }).next_cursor).toBe('abc')
    const upload = { url: 'http://localhost:4566/x', method: 'PUT', headers: { 'Content-Type': 'application/pdf' }, expires_at: '2026-11-22T10:15:00Z' }
    expect(recruitment.createdApplication.parse({ ...app, resume_upload: upload }).resume_upload.method).toBe('PUT')
  })

  it('recrutamento: entrevista com scorecard', () => {
    const sc = { id: id(5), interview_id: id(4), evaluator_id: id(6), recommendation: 'strong_yes', ratings: [{ criterion: 'x', score: 5 }], notes: null }
    const iv = { id: id(4), application_id: id(2), kind: 'entrevista-rh', interviewer_id: null, scheduled_at: '2026-11-23T10:00:00+00:00', scorecards: [sc] }
    expect(recruitment.interview.parse(iv).scorecards).toHaveLength(1)
  })

  it('engajamento: pesquisa com already_answered, resultados com supressão e agenda de 1:1', () => {
    const survey = { id: id(7), title: 'Clima', status: 'open', questions: [{ id: 'q1', text: 'ok' }], segments: ['obras'], min_group_size: 5, already_answered: false }
    expect(engagement.surveys.parse([survey])[0]?.already_answered).toBe(false)
    const results = { survey_id: id(7), min_group_size: 5, participation: 9, suppressed: 1, segments: [{ segment_key: '_all', respondents: 9, scores: { q1: 4.2 }, comments: ['bom'] }] }
    expect(engagement.surveyResults.parse(results).suppressed).toBe(1)
    const meeting = { id: id(8), leader_id: id(1), participant_id: id(2), scheduled_for: '2026-11-23T10:00:00.000Z', agenda: [{ id: id(9), text: 'x', added_by: id(1), done: false }] }
    expect(engagement.meetingContext.parse({ meeting, recent_feedbacks: [], recent_praises: [], open_agenda: meeting.agenda }).open_agenda).toHaveLength(1)
  })

  it('analytics: turnover com total nulo e nine box', () => {
    const report = { period: '2026-Q3', group_by: 'area', min_group_size: 5, groups: [], suppressed_groups: 2, total: null }
    expect(analytics.turnoverReport.parse(report).total).toBeNull()
    expect(analytics.nineBoxDistribution.parse({ cycle_id: id(1), min_group_size: 5, quadrants: [{ quadrant: 'Q9', count: 6 }], suppressed_quadrants: 3 }).quadrants).toHaveLength(1)
  })

  it('recusa resposta fora do contrato', () => {
    expect(recruitment.application.safeParse({ id: 'x' }).success).toBe(false)
    expect(analytics.turnoverReport.safeParse({ period: '2026', group_by: 'banana' }).success).toBe(false)
  })
})
