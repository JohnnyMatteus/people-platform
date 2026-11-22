package com.peopleplatform.performance.integration

import org.junit.jupiter.api.Tag
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc
import org.springframework.boot.test.context.SpringBootTest
import org.springframework.test.context.ActiveProfiles
import org.springframework.test.context.DynamicPropertyRegistry
import org.springframework.test.context.DynamicPropertySource
import org.testcontainers.containers.PostgreSQLContainer
import java.sql.DriverManager
import java.util.UUID

/**
 * Postgres real para os testes de integração. Em CI o serviço `postgres` do job é usado via
 * TEST_PG_URL (jdbc:postgresql://host:5432/postgres, com TEST_PG_USER/TEST_PG_PASSWORD); sem a
 * variável, sobe um container com Testcontainers.
 *
 * As migrações rodam como superusuário; a aplicação conecta como `perf_app`, membro de
 * `performance_app` e sem BYPASSRLS, exatamente como em produção.
 */
@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
@Tag("integration")
abstract class IntegrationTestBase {
    companion object {
        private const val APP_USER = "perf_app"
        private const val APP_PASSWORD = "perf-app-test"

        private val admin: Triple<String, String, String> by lazy {
            val url = System.getenv("TEST_PG_URL")
            if (url != null) {
                Triple(
                    url,
                    System.getenv("TEST_PG_USER") ?: "postgres",
                    System.getenv("TEST_PG_PASSWORD") ?: "postgres",
                )
            } else {
                val container = PostgreSQLContainer("postgres:17.6-alpine").also { it.start() }
                Triple(container.jdbcUrl, container.username, container.password)
            }
        }

        private val database: String by lazy {
            val name = "perf_" + UUID.randomUUID().toString().replace("-", "").take(12)
            DriverManager.getConnection(admin.first, admin.second, admin.third).use { c ->
                c.createStatement().use {
                    it.execute(
                        """
                        DO ${'$'}${'$'} BEGIN
                          IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'performance_app') THEN
                            CREATE ROLE performance_app NOLOGIN;
                          END IF;
                          IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '$APP_USER') THEN
                            CREATE ROLE $APP_USER LOGIN PASSWORD '$APP_PASSWORD' IN ROLE performance_app;
                          END IF;
                        END ${'$'}${'$'}
                        """.trimIndent(),
                    )
                    it.execute("CREATE DATABASE $name")
                }
            }
            name
        }

        private fun urlFor(db: String) = admin.first.substringBeforeLast('/') + "/" + db.substringBefore('?')

        @JvmStatic
        @DynamicPropertySource
        fun properties(registry: DynamicPropertyRegistry) {
            val url = urlFor(database)
            registry.add("spring.datasource.url") { url }
            registry.add("spring.datasource.username") { APP_USER }
            registry.add("spring.datasource.password") { APP_PASSWORD }
            registry.add("spring.flyway.url") { url }
            registry.add("spring.flyway.user") { admin.second }
            registry.add("spring.flyway.password") { admin.third }
        }
    }
}
