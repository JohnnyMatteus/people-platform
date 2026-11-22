<?php

namespace Tests\Support;

use App\Infrastructure\Auth\JwtVerifier;
use App\Infrastructure\Auth\KeySetProvider;
use Firebase\JWT\JWT;
use Firebase\JWT\Key;

/** Gera JWTs RS256 como o Keycloak do realm people e instala a chave pública no verificador. */
trait IssuesTokens
{
    private static ?\OpenSSLAsymmetricKey $signingKey = null;

    protected function signingKey(): \OpenSSLAsymmetricKey
    {
        return self::$signingKey ??= openssl_pkey_new(['private_key_bits' => 2048, 'private_key_type' => OPENSSL_KEYTYPE_RSA]);
    }

    protected function trustTestKey(string $kid = 'test-key'): FakeKeySetProvider
    {
        $public = openssl_pkey_get_details($this->signingKey())['key'];
        $provider = new FakeKeySetProvider([$kid => new Key($public, 'RS256')]);
        $this->app->instance(KeySetProvider::class, $provider);
        $this->app->forgetInstance(JwtVerifier::class);

        return $provider;
    }

    /** @param array<string, mixed> $override */
    protected function tokenFor(string $tenantId, array $roles = ['manager'], ?string $sub = null, array $override = [], string $kid = 'test-key'): string
    {
        $claims = [
            'iss' => 'http://keycloak.test/realms/people',
            'aud' => ['people-api'],
            'sub' => $sub ?? '00000000-0000-7000-8000-0000000000aa',
            'tenant_id' => $tenantId,
            'realm_access' => ['roles' => $roles],
            'iat' => time(),
            'exp' => time() + 300,
            ...$override,
        ];

        return JWT::encode($claims, $this->signingKey(), 'RS256', $kid);
    }

    /** @return array<string, string> */
    protected function bearer(string $tenantId, array $roles = ['manager'], ?string $sub = null): array
    {
        return ['Authorization' => 'Bearer '.$this->tokenFor($tenantId, $roles, $sub)];
    }
}
