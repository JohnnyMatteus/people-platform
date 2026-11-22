package com.peopleplatform.performance.adapters

import com.peopleplatform.performance.adapters.web.AudienceValidator
import com.peopleplatform.performance.adapters.web.TenantClaimValidator
import com.peopleplatform.performance.adapters.web.toCaller
import org.springframework.security.oauth2.jwt.Jwt
import java.time.Instant
import java.util.UUID
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class TokenValidatorsTest {
    private fun jwt(subject: String?, tenant: Any?, aud: List<String> = listOf("people-api")): Jwt =
        Jwt.withTokenValue("t").header("alg", "none").issuedAt(Instant.now()).expiresAt(Instant.now().plusSeconds(60))
            .audience(aud).claim("realm_access", mapOf("roles" to listOf("manager")))
            .apply {
                if (subject != null) subject(subject)
                if (tenant != null) claim("tenant_id", tenant)
            }.build()

    @Test
    fun `token sem tenant_id ou com sub invalido e recusado`() {
        val v = TenantClaimValidator()
        val id = UUID.randomUUID().toString()
        assertTrue(v.validate(jwt(id, null)).hasErrors())
        assertTrue(v.validate(jwt(id, "nao-e-uuid")).hasErrors())
        assertTrue(v.validate(jwt("ana", id)).hasErrors())
        assertFalse(v.validate(jwt(id, id)).hasErrors())
    }

    @Test
    fun `audiencia diferente da esperada e recusada`() {
        val id = UUID.randomUUID().toString()
        assertFalse(AudienceValidator("people-api").validate(jwt(id, id)).hasErrors())
        assertTrue(AudienceValidator("outra-api").validate(jwt(id, id)).hasErrors())
    }

    @Test
    fun `o chamador vem so do token`() {
        val tenant = UUID.randomUUID()
        val caller = jwt(UUID.randomUUID().toString(), tenant.toString()).toCaller()
        assertEquals(tenant, caller.tenantId.value)
        assertTrue(caller.isManager)
        assertFalse(caller.isAdmin)
    }
}
