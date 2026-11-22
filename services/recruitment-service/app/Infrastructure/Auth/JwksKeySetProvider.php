<?php

declare(strict_types=1);

namespace App\Infrastructure\Auth;

use Firebase\JWT\JWK;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Facades\Http;

final class JwksKeySetProvider implements KeySetProvider
{
    public function __construct(private readonly string $jwksUrl, private readonly int $ttlSeconds) {}

    public function keys(bool $refresh = false): array
    {
        if ($refresh) {
            Cache::forget('oidc.jwks');
        }

        $jwks = Cache::remember(
            'oidc.jwks',
            $this->ttlSeconds,
            fn () => Http::timeout(5)->retry(2, 200)->get($this->jwksUrl)->throw()->json(),
        );

        return JWK::parseKeySet($jwks, 'RS256');
    }
}
