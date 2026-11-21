<?php

declare(strict_types=1);

namespace App\Infrastructure\Auth;

use App\Domain\Principal;
use Firebase\JWT\JWT;
use Firebase\JWT\Key;
use Throwable;
use UnexpectedValueException;

/** Valida o JWT emitido pelo Keycloak (realm people): assinatura RS256, issuer, audience e tenant_id. */
final class JwtVerifier
{
    private const UUID = '/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i';

    public function __construct(
        private readonly KeySetProvider $keys,
        private readonly string $issuer,
        private readonly string $audience,
    ) {}

    public function verify(string $token): Principal
    {
        $claims = $this->decode($token);

        if (($claims['iss'] ?? null) !== $this->issuer) {
            throw new UnauthenticatedException('Emissor do token inválido.');
        }
        if (! in_array($this->audience, (array) ($claims['aud'] ?? []), true)) {
            throw new UnauthenticatedException('Audiência do token inválida.');
        }

        $tenantId = $claims['tenant_id'] ?? null;
        if (! is_string($tenantId) || ! preg_match(self::UUID, $tenantId)) {
            throw new UnauthenticatedException('Claim tenant_id ausente ou inválida.');
        }
        $userId = $claims['sub'] ?? null;
        if (! is_string($userId) || ! preg_match(self::UUID, $userId)) {
            throw new UnauthenticatedException('Claim sub ausente ou inválida.');
        }

        return new Principal($userId, strtolower($tenantId), array_values((array) ($claims['realm_access']['roles'] ?? [])));
    }

    /** @return array<string, mixed> */
    private function decode(string $token): array
    {
        JWT::$leeway = 30;

        try {
            return $this->claims($token, $this->keys->keys());
        } catch (UnexpectedValueException $e) {
            // kid desconhecido pode ser rotação de chaves: tenta uma vez com o JWKS atualizado.
            if (! str_contains($e->getMessage(), '"kid"')) {
                throw new UnauthenticatedException('Token inválido.', previous: $e);
            }
        } catch (Throwable $e) {
            throw new UnauthenticatedException('Token inválido.', previous: $e);
        }

        try {
            return $this->claims($token, $this->keys->keys(refresh: true));
        } catch (Throwable $e) {
            throw new UnauthenticatedException('Token inválido.', previous: $e);
        }
    }

    /**
     * @param  array<string, Key>  $keys
     * @return array<string, mixed>
     */
    private function claims(string $token, array $keys): array
    {
        return json_decode(json_encode(JWT::decode($token, $keys), JSON_THROW_ON_ERROR), true, flags: JSON_THROW_ON_ERROR);
    }
}
