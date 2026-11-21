"""Autenticação por JWT. O tenant vem da claim, nunca da requisição."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Annotated, Any, Protocol
from uuid import UUID

import jwt
from fastapi import Depends, HTTPException, Request

ALLOWED_ROLES = frozenset({"manager", "tenant-admin", "platform-admin"})


@dataclass(frozen=True, slots=True)
class Principal:
    subject: str
    tenant_id: UUID
    roles: frozenset[str]


class TokenVerifier(Protocol):
    def verify(self, token: str) -> Principal: ...


class InvalidTokenError(Exception):
    pass


def principal_from_claims(claims: dict[str, Any]) -> Principal:
    try:
        tenant_id = UUID(str(claims["tenant_id"]))
    except (KeyError, ValueError) as exc:
        raise InvalidTokenError("claim tenant_id ausente ou inválida") from exc
    roles = frozenset(claims.get("realm_access", {}).get("roles", []))
    return Principal(str(claims.get("sub", "")), tenant_id, roles)


class JwksVerifier:
    def __init__(self, *, jwks_url: str, issuer: str, audience: str) -> None:
        self._jwks = jwt.PyJWKClient(jwks_url, cache_keys=True)
        self._issuer, self._audience = issuer, audience

    def verify(self, token: str) -> Principal:
        try:
            key = self._jwks.get_signing_key_from_jwt(token).key
            claims = jwt.decode(
                token, key, algorithms=["RS256"], issuer=self._issuer, audience=self._audience
            )
        except jwt.PyJWTError as exc:
            raise InvalidTokenError(str(exc)) from exc
        return principal_from_claims(claims)


def current_principal(request: Request) -> Principal:
    header = request.headers.get("authorization", "")
    scheme, _, token = header.partition(" ")
    if scheme.lower() != "bearer" or not token:
        raise HTTPException(401, detail="unauthorized")
    verifier: TokenVerifier = request.app.state.verifier
    try:
        principal = verifier.verify(token)
    except InvalidTokenError as exc:
        raise HTTPException(401, detail="unauthorized") from exc
    if not principal.roles & ALLOWED_ROLES:
        raise HTTPException(403, detail="forbidden")
    return principal


CurrentPrincipal = Annotated[Principal, Depends(current_principal)]
