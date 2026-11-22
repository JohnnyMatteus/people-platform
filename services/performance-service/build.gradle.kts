// O domínio (com.peopleplatform.performance.domain) continua Kotlin puro; Spring Boot, JPA e
// Kafka ficam nos adaptadores em volta dele (ADR 0011).
plugins {
    kotlin("jvm") version "2.1.21"
    kotlin("plugin.spring") version "2.1.21"
    kotlin("plugin.jpa") version "2.1.21"
    id("org.springframework.boot") version "3.5.7"
    id("io.spring.dependency-management") version "1.1.7"
    id("org.jlleitschuh.gradle.ktlint") version "12.3.0"
}

group = "com.peopleplatform"
version = "0.2.0"

repositories {
    mavenCentral()
}

kotlin {
    jvmToolchain(21)
    compilerOptions {
        allWarningsAsErrors.set(true)
        freeCompilerArgs.addAll("-Xjsr305=strict")
    }
}

dependencies {
    implementation("org.springframework.boot:spring-boot-starter-web")
    implementation("org.springframework.boot:spring-boot-starter-validation")
    implementation("org.springframework.boot:spring-boot-starter-data-jpa")
    implementation("org.springframework.boot:spring-boot-starter-oauth2-resource-server")
    implementation("org.springframework.boot:spring-boot-starter-actuator")
    implementation("org.springframework.kafka:spring-kafka")
    implementation("org.flywaydb:flyway-core")
    implementation("org.flywaydb:flyway-database-postgresql")
    implementation("com.fasterxml.jackson.module:jackson-module-kotlin")
    implementation("org.jetbrains.kotlin:kotlin-reflect")
    implementation("io.micrometer:micrometer-registry-prometheus")
    runtimeOnly("org.postgresql:postgresql")

    testImplementation("org.springframework.boot:spring-boot-starter-test")
    testImplementation("org.springframework.security:spring-security-test")
    testImplementation("org.springframework.boot:spring-boot-testcontainers")
    testImplementation("org.testcontainers:junit-jupiter")
    testImplementation("org.testcontainers:postgresql")
    testImplementation("com.networknt:json-schema-validator:1.5.8")
    testImplementation(kotlin("test"))
    testImplementation("org.junit.jupiter:junit-jupiter-params")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.test {
    useJUnitPlatform {
        // Integração (Postgres real via Testcontainers) roda em tarefa própria: precisa de Docker.
        excludeTags("integration")
    }
    testLogging {
        events("passed", "failed", "skipped")
    }
}

val integrationTest by tasks.registering(Test::class) {
    description = "Testes com Postgres real (Testcontainers): RLS, vazamento entre tenants, outbox e contrato."
    group = "verification"
    testClassesDirs = sourceSets.test.get().output.classesDirs
    classpath = sourceSets.test.get().runtimeClasspath
    useJUnitPlatform { includeTags("integration") }
    testLogging { events("passed", "failed", "skipped") }
    shouldRunAfter(tasks.test)
}

tasks.bootJar {
    archiveFileName.set("performance-service.jar")
}

tasks.jar {
    enabled = false
}
