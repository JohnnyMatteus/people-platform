<?php

use App\Http\Controllers\ApplicationController;
use App\Http\Controllers\InterviewController;
use App\Http\Controllers\JobController;
use Illuminate\Support\Facades\Route;

// Dados de candidatos são pessoais (LGPD): só quem conduz seleção acessa.
Route::middleware(['jwt', 'role:tenant-admin,manager', 'tenant'])->prefix('v1')->group(function () {
    Route::get('pipeline-template', [JobController::class, 'showTemplate']);
    Route::put('pipeline-template', [JobController::class, 'replaceTemplate']);

    Route::get('jobs', [JobController::class, 'index']);
    Route::post('jobs', [JobController::class, 'store']);
    Route::get('jobs/{jobId}', [JobController::class, 'show'])->whereUuid('jobId');
    Route::get('jobs/{jobId}/applications', [ApplicationController::class, 'index'])->whereUuid('jobId');
    Route::post('jobs/{jobId}/applications', [ApplicationController::class, 'store'])->whereUuid('jobId');

    Route::get('applications/{applicationId}', [ApplicationController::class, 'show'])->whereUuid('applicationId');
    Route::put('applications/{applicationId}/stage', [ApplicationController::class, 'moveStage'])->whereUuid('applicationId');
    Route::post('applications/{applicationId}/reject', [ApplicationController::class, 'reject'])->whereUuid('applicationId');
    Route::get('applications/{applicationId}/resume', [ApplicationController::class, 'resume'])->whereUuid('applicationId');
    Route::post('applications/{applicationId}/interviews', [InterviewController::class, 'store'])->whereUuid('applicationId');

    Route::post('interviews/{interviewId}/scorecards', [InterviewController::class, 'submitScorecard'])->whereUuid('interviewId');
});
