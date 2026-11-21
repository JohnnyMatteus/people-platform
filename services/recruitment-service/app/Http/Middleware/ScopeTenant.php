<?php

declare(strict_types=1);

namespace App\Http\Middleware;

use App\Domain\Principal;
use App\Infrastructure\Tenancy\TenantTransaction;
use Closure;
use Illuminate\Http\Request;
use Symfony\Component\HttpFoundation\Response;

/** Uma transação por requisição, com o escopo de RLS definido a partir do tenant do token. */
final class ScopeTenant
{
    public function __construct(private readonly TenantTransaction $transactions) {}

    public function handle(Request $request, Closure $next): Response
    {
        /** @var Principal $principal */
        $principal = $request->attributes->get('principal');

        return $this->transactions->run($principal->tenantId, fn () => $next($request));
    }
}
