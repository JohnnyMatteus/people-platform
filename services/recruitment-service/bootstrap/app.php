<?php

use App\Domain\ConflictException;
use App\Domain\DomainRuleViolation;
use App\Http\Middleware\AuthenticateJwt;
use App\Http\Middleware\RequireRole;
use App\Http\Middleware\ScopeTenant;
use App\Infrastructure\Auth\UnauthenticatedException;
use Illuminate\Database\Eloquent\ModelNotFoundException;
use Illuminate\Foundation\Application;
use Illuminate\Foundation\Configuration\Exceptions;
use Illuminate\Foundation\Configuration\Middleware;
use Illuminate\Http\JsonResponse;
use Illuminate\Validation\ValidationException;
use Symfony\Component\HttpKernel\Exception\HttpExceptionInterface;

$problem = static fn (int $status, string $title, ?string $detail = null, array $extra = [], array $headers = []): JsonResponse => response()->json(
    ['type' => 'about:blank', 'title' => $title, 'status' => $status, 'detail' => $detail, ...$extra],
    $status,
    ['Content-Type' => 'application/problem+json', ...$headers],
);

return Application::configure(basePath: dirname(__DIR__))
    ->withRouting(
        api: __DIR__.'/../routes/api.php',
        commands: __DIR__.'/../routes/console.php',
        health: '/up',
        apiPrefix: '',
    )
    ->withMiddleware(function (Middleware $middleware): void {
        $middleware->alias(['jwt' => AuthenticateJwt::class, 'role' => RequireRole::class, 'tenant' => ScopeTenant::class]);
    })
    ->withExceptions(function (Exceptions $exceptions) use ($problem): void {
        $exceptions->shouldRenderJsonWhen(static fn () => true);
        $exceptions->dontReport([UnauthenticatedException::class, DomainRuleViolation::class, ConflictException::class]);

        $exceptions->render(fn (UnauthenticatedException $e) => $problem(401, 'Não autenticado', $e->getMessage(), headers: ['WWW-Authenticate' => 'Bearer']));
        $exceptions->render(fn (ValidationException $e) => $problem(422, 'Requisição inválida', null, ['errors' => $e->errors()]));
        $exceptions->render(fn (DomainRuleViolation $e) => $problem(422, 'Regra de negócio violada', $e->getMessage()));
        $exceptions->render(fn (ConflictException $e) => $problem(409, 'Conflito de estado', $e->getMessage()));
        $exceptions->render(fn (ModelNotFoundException $e) => $problem(404, 'Não encontrado'));
        $exceptions->render(fn (HttpExceptionInterface $e) => $problem($e->getStatusCode(), match ($e->getStatusCode()) {
            403 => 'Acesso negado',
            404 => 'Não encontrado',
            405 => 'Método não permitido',
            default => 'Erro na requisição',
        }, $e->getStatusCode() === 403 ? $e->getMessage() : null));
    })->create();
