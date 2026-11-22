package com.peopleplatform.performance.adapters.web

import com.peopleplatform.performance.app.Caller
import com.peopleplatform.performance.domain.shared.EmployeeId
import com.peopleplatform.performance.domain.shared.TenantId
import org.springframework.beans.factory.annotation.Value
import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration
import org.springframework.core.convert.converter.Converter
import org.springframework.http.HttpMethod
import org.springframework.http.HttpStatus
import org.springframework.security.authentication.AbstractAuthenticationToken
import org.springframework.security.config.annotation.web.builders.HttpSecurity
import org.springframework.security.config.annotation.web.configuration.EnableWebSecurity
import org.springframework.security.config.annotation.web.invoke
import org.springframework.security.config.http.SessionCreationPolicy
import org.springframework.security.core.GrantedAuthority
import org.springframework.security.core.authority.SimpleGrantedAuthority
import org.springframework.security.oauth2.core.OAuth2Error
import org.springframework.security.oauth2.core.OAuth2TokenValidator
import org.springframework.security.oauth2.core.OAuth2TokenValidatorResult
import org.springframework.security.oauth2.jwt.Jwt
import org.springframework.security.oauth2.jwt.JwtDecoder
import org.springframework.security.oauth2.jwt.JwtValidators
import org.springframework.security.oauth2.jwt.NimbusJwtDecoder
import org.springframework.security.oauth2.server.resource.authentication.JwtAuthenticationToken
import org.springframework.security.web.SecurityFilterChain
import org.springframework.security.web.authentication.HttpStatusEntryPoint
import java.util.UUID

/** O token precisa carregar o tenant (UUID) e o sujeito (UUID): sem isso nada é autorizado. */
class TenantClaimValidator : OAuth2TokenValidator<Jwt> {
    override fun validate(token: Jwt): OAuth2TokenValidatorResult {
        val ok = token.getClaimAsString("tenant_id").isUuid() && token.subject.isUuid()
        return if (ok) {
            OAuth2TokenValidatorResult.success()
        } else {
            OAuth2TokenValidatorResult.failure(
                OAuth2Error("invalid_token", "claims tenant_id e sub devem ser UUID", null),
            )
        }
    }

    private fun String?.isUuid(): Boolean = this != null && runCatching { UUID.fromString(this) }.isSuccess
}

class AudienceValidator(private val audience: String) : OAuth2TokenValidator<Jwt> {
    override fun validate(token: Jwt): OAuth2TokenValidatorResult =
        if (audience in (token.audience ?: emptyList())) {
            OAuth2TokenValidatorResult.success()
        } else {
            OAuth2TokenValidatorResult.failure(OAuth2Error("invalid_token", "audiência inválida", null))
        }
}

fun Jwt.toCaller() = Caller(
    tenantId = TenantId(UUID.fromString(getClaimAsString("tenant_id"))),
    employeeId = EmployeeId(UUID.fromString(subject)),
    name = getClaimAsString("name"),
    roles = realmRoles(),
)

private fun Jwt.realmRoles(): Set<String> =
    (getClaimAsMap("realm_access")?.get("roles") as? Collection<*>)?.filterIsInstance<String>()?.toSet() ?: emptySet()

private class RealmRoleConverter : Converter<Jwt, AbstractAuthenticationToken> {
    override fun convert(source: Jwt): AbstractAuthenticationToken {
        val authorities: List<GrantedAuthority> = source.realmRoles().map { SimpleGrantedAuthority("ROLE_$it") }
        return JwtAuthenticationToken(source, authorities)
    }
}

@Configuration
@EnableWebSecurity
class SecurityConfig {
    @Bean
    fun jwtDecoder(
        @Value("\${performance.security.jwks-uri}") jwksUri: String,
        @Value("\${performance.security.issuer}") issuer: String,
        @Value("\${performance.security.audience}") audience: String,
    ): JwtDecoder {
        val decoder = NimbusJwtDecoder.withJwkSetUri(jwksUri).build()
        decoder.setJwtValidator(
            org.springframework.security.oauth2.core.DelegatingOAuth2TokenValidator(
                JwtValidators.createDefaultWithIssuer(issuer), AudienceValidator(audience), TenantClaimValidator(),
            ),
        )
        return decoder
    }

    @Bean
    fun filterChain(http: HttpSecurity): SecurityFilterChain {
        val managers = arrayOf(Caller.ROLE_MANAGER, Caller.ROLE_ADMIN)
        http {
            csrf { disable() }
            sessionManagement { sessionCreationPolicy = SessionCreationPolicy.STATELESS }
            authorizeHttpRequests {
                authorize("/actuator/health/**", permitAll)
                authorize("/actuator/prometheus", permitAll)
                authorize(HttpMethod.PUT, "/v1/nine-box/policy", hasRole(Caller.ROLE_ADMIN))
                authorize(HttpMethod.POST, "/v1/cycles", hasRole(Caller.ROLE_ADMIN))
                authorize(HttpMethod.POST, "/v1/cycles/*/open", hasRole(Caller.ROLE_ADMIN))
                authorize(HttpMethod.POST, "/v1/cycles/*/evaluations/manager", hasAnyRole(*managers))
                authorize(HttpMethod.GET, "/v1/cycles/*/evaluations", hasAnyRole(*managers))
                authorize("/v1/cycles/*/calibration-sessions/**", hasAnyRole(*managers))
                authorize("/v1/calibration-sessions/**", hasAnyRole(*managers))
                authorize("/v1/critical-positions/**", hasAnyRole(*managers))
                authorize("/v1/employees", hasAnyRole(*managers))
                authorize(anyRequest, authenticated)
            }
            oauth2ResourceServer { jwt { jwtAuthenticationConverter = RealmRoleConverter() } }
            exceptionHandling { authenticationEntryPoint = HttpStatusEntryPoint(HttpStatus.UNAUTHORIZED) }
        }
        return http.build()
    }
}
