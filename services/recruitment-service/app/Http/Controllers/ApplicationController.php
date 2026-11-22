<?php

declare(strict_types=1);

namespace App\Http\Controllers;

use App\Application\ApplicationService;
use App\Http\Presenter;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

final class ApplicationController extends Controller
{
    public function __construct(private readonly ApplicationService $applications) {}

    public function index(string $jobId): JsonResponse
    {
        $page = $this->applications->listForJob($jobId);

        return response()->json([
            'items' => collect($page->items())->map(Presenter::application(...))->all(),
            'next_cursor' => $page->nextCursor()?->encode(),
        ]);
    }

    public function store(Request $request, string $jobId): JsonResponse
    {
        $data = $request->validate([
            'name' => ['required', 'string', 'max:200'],
            'email' => ['required', 'email:rfc', 'max:254'],
            'resume_content_type' => ['nullable', 'string', 'max:120'],
        ]);

        $result = $this->applications->apply(
            $jobId,
            $data['name'],
            $data['email'],
            $data['resume_content_type'] ?? 'application/pdf',
        );

        return response()->json([
            ...Presenter::application($result['application']),
            'resume_upload' => $result['resume_upload'],
        ], 201);
    }

    public function show(string $applicationId): JsonResponse
    {
        return response()->json(Presenter::application($this->applications->find($applicationId)));
    }

    public function moveStage(Request $request, string $applicationId): JsonResponse
    {
        $data = $request->validate([
            'stage' => ['required', 'string', 'max:60'],
            'start_date' => ['nullable', 'date_format:Y-m-d'],
        ]);

        $application = $this->applications->moveStage($applicationId, $data['stage'], $data['start_date'] ?? null);

        return response()->json(Presenter::application($application));
    }

    public function reject(string $applicationId): JsonResponse
    {
        return response()->json(Presenter::application($this->applications->reject($applicationId)));
    }

    public function resume(string $applicationId): JsonResponse
    {
        return response()->json(['url' => $this->applications->resumeDownloadUrl($applicationId)]);
    }
}
