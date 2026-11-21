<?php

declare(strict_types=1);

namespace App\Application;

use App\Domain\ConflictException;
use App\Domain\Recommendation;
use App\Models\Interview;
use App\Models\JobApplication;
use App\Models\Scorecard;
use Illuminate\Database\UniqueConstraintViolationException;

final class InterviewService
{
    public function schedule(string $applicationId, string $kind, string $scheduledAt, ?string $interviewerId): Interview
    {
        $application = JobApplication::query()->findOrFail($applicationId);
        if ($application->status->isClosed()) {
            throw new ConflictException('A candidatura já foi encerrada.');
        }

        return Interview::create([
            'application_id' => $application->id,
            'kind' => $kind,
            'scheduled_at' => $scheduledAt,
            'interviewer_id' => $interviewerId,
        ]);
    }

    /** @param list<array{criterion: string, score: int}> $ratings */
    public function submitScorecard(
        string $interviewId,
        string $evaluatorId,
        Recommendation $recommendation,
        array $ratings,
        ?string $notes,
    ): Scorecard {
        $interview = Interview::query()->findOrFail($interviewId);

        try {
            return Scorecard::create([
                'interview_id' => $interview->id,
                'evaluator_id' => $evaluatorId,
                'recommendation' => $recommendation,
                'ratings' => $ratings,
                'notes' => $notes,
            ]);
        } catch (UniqueConstraintViolationException) {
            throw new ConflictException('Este avaliador já enviou o scorecard desta entrevista.');
        }
    }
}
