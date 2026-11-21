// Nesta rodada o módulo contém só o núcleo de domínio (Kotlin puro, sem Spring), para que as
// regras de nine box e sucessão sejam testadas sem framework. A camada Spring Boot (API HTTP,
// JPA e consumo de eventos) entra na próxima rodada como adaptadores em volta deste núcleo.
plugins {
    kotlin("jvm") version "2.1.21"
    id("org.jlleitschuh.gradle.ktlint") version "12.3.0"
}

group = "com.peopleplatform"
version = "0.1.0"

repositories {
    mavenCentral()
}

kotlin {
    jvmToolchain(21)
    compilerOptions {
        allWarningsAsErrors.set(true)
    }
}

dependencies {
    testImplementation(kotlin("test"))
    testImplementation(platform("org.junit:junit-bom:5.12.2"))
    testImplementation("org.junit.jupiter:junit-jupiter-params")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.test {
    useJUnitPlatform()
    testLogging {
        events("passed", "failed", "skipped")
    }
}
