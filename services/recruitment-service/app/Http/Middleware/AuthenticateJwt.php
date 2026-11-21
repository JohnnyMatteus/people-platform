<?php

declare(strict_types=1);

namespace App\Http\Middleware;

use App\Infrastructure\Auth\JwtVerifier;
use App\Infrastructure\Auth\UnauthenticatedException;
use Closure;
use Illuminate\Http\Request;
use Symfony\Component\HttpFoundation\Response;

final class AuthenticateJwt
{
    public function __construct(private readonly JwtVerifier $verifier) {}

    public function handle(Request $request, Closure $next): Response
    {
        $token = $request->bearerToken();
        if ($token === null) {
            throw new UnauthenticatedException('Token ausente.');
        }

        $request->attributes->set('principal', $this->verifier->verify($token));

        return $next($request);
    }
}
