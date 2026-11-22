<?php

declare(strict_types=1);

namespace App\Http\Controllers;

use App\Application\InterviewService;
use App\Domain\Principal;
use App\Domain\Recommendation;
use App\Http\Presenter;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Validation\Rule;

final class InterviewController extends Controller
{
    public function __construct(private readonly InterviewService $interviews) {}

    public function store(Request $request, string $applicationId): JsonResponse
    {
        $data = $request->validate([
            'kind' => ['required', 'string', 'max:60'],
            'scheduled_at' => ['required', 'date'],
            'interviewer_id' => ['nullable', 'uuid'],
        ]);

        $interview = $this->interviews->schedule(
            $applicationId,
            $data['kind'],
            $data['scheduled_at'],
            $data['interviewer_id'] ?? null,
        );

        return response()->json(Presenter::interview($interview), 201);
    }

    public function submitScorecard(Request $request, string $interviewId): JsonResponse
    {
        /** @var Principal $principal */
        $principal = $request->attributes->get('principal');

        $data = $request->validate([
            'recommendation' => ['required', Rule::enum(Recommendation::class)],
            'ratings' => ['required', 'array', 'min:1', 'max:20'],
            'ratings.*.criterion' => ['required', 'string', 'max:120'],
            'ratings.*.score' => ['required', 'integer', 'between:1,5'],
            'notes' => ['nullable', 'string', 'max:4000'],
        ]);

        $scorecard = $this->interviews->submitScorecard(
            $interviewId,
            $principal->userId,
            Recommendation::from($data['recommendation']),
            $data['ratings'],
            $data['notes'] ?? null,
        );

        return response()->json(Presenter::scorecard($scorecard), 201);
    }
}
