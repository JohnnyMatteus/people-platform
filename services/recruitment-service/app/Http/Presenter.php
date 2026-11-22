<?php

declare(strict_types=1);

namespace App\Http;

use App\Models\Interview;
use App\Models\Job;
use App\Models\JobApplication;
use App\Models\Scorecard;

/** Formato JSON da API (contracts/openapi/recruitment-service.yaml). */
final class Presenter
{
    /** @return array<string, mixed> */
    public static function job(Job $job): array
    {
        return [
            'id' => $job->id,
            'title' => $job->title,
            'unit' => $job->unit,
            'status' => $job->status,
            'stages' => $job->stages,
            'created_at' => $job->created_at?->toIso8601String(),
        ];
    }

    /** @return array<string, mixed> */
    public static function application(JobApplication $application): array
    {
        return [
            'id' => $application->id,
            'job_id' => $application->job_id,
            'stage' => $application->stage,
            'status' => $application->status->value,
            'start_date' => $application->start_date?->format('Y-m-d'),
            'candidate' => $application->relationLoaded('candidate') ? [
                'id' => $application->candidate->id,
                'name' => $application->candidate->name,
                'email' => $application->candidate->email,
            ] : null,
            'interviews' => $application->relationLoaded('interviews')
                ? $application->interviews->map(self::interview(...))->all()
                : null,
        ];
    }

    /** @return array<string, mixed> */
    public static function interview(Interview $interview): array
    {
        return [
            'id' => $interview->id,
            'application_id' => $interview->application_id,
            'kind' => $interview->kind,
            'interviewer_id' => $interview->interviewer_id,
            'scheduled_at' => $interview->scheduled_at->utc()->toIso8601String(),
            'scorecards' => $interview->relationLoaded('scorecards')
                ? $interview->scorecards->map(self::scorecard(...))->all()
                : null,
        ];
    }

    /** @return array<string, mixed> */
    public static function scorecard(Scorecard $scorecard): array
    {
        return [
            'id' => $scorecard->id,
            'interview_id' => $scorecard->interview_id,
            'evaluator_id' => $scorecard->evaluator_id,
            'recommendation' => $scorecard->recommendation->value,
            'ratings' => $scorecard->ratings,
            'notes' => $scorecard->notes,
        ];
    }
}
