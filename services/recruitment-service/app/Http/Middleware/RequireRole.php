<?php

declare(strict_types=1);

namespace App\Http\Middleware;

use App\Domain\Principal;
use Closure;
use Illuminate\Http\Request;
use Symfony\Component\HttpFoundation\Response;
use Symfony\Component\HttpKernel\Exception\AccessDeniedHttpException;

final class RequireRole
{
    public function handle(Request $request, Closure $next, string ...$roles): Response
    {
        /** @var Principal $principal */
        $principal = $request->attributes->get('principal');

        if (! $principal->hasAnyRole(...$roles)) {
            throw new AccessDeniedHttpException('Papel sem permissão para esta operação.');
        }

        return $next($request);
    }
}
