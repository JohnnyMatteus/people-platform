<?php

declare(strict_types=1);

namespace App\Http\Controllers;

use App\Application\JobService;
use App\Application\PipelineTemplateService;
use App\Http\Presenter;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

final class JobController extends Controller
{
    public function __construct(private readonly JobService $jobs, private readonly PipelineTemplateService $templates) {}

    public function index(): JsonResponse
    {
        $page = $this->jobs->list();

        return response()->json([
            'items' => collect($page->items())->map(Presenter::job(...))->all(),
            'next_cursor' => $page->nextCursor()?->encode(),
        ]);
    }

    public function store(Request $request): JsonResponse
    {
        $data = $request->validate([
            'title' => ['required', 'string', 'max:200'],
            'unit' => ['nullable', 'string', 'max:120'],
            'stages' => ['nullable', 'array', 'min:2', 'max:12'],
            'stages.*' => ['string', 'max:60'],
        ]);

        $job = $this->jobs->open($data['title'], $data['unit'] ?? '', $data['stages'] ?? null);

        return response()->json(Presenter::job($job), 201);
    }

    public function show(string $jobId): JsonResponse
    {
        return response()->json(Presenter::job($this->jobs->find($jobId)));
    }

    public function showTemplate(): JsonResponse
    {
        return response()->json(['stages' => $this->templates->current()->stages()]);
    }

    public function replaceTemplate(Request $request): JsonResponse
    {
        $data = $request->validate([
            'stages' => ['required', 'array', 'min:2', 'max:12'],
            'stages.*' => ['string', 'max:60'],
        ]);

        return response()->json(['stages' => $this->templates->replace($data['stages'])->stages()]);
    }
}
