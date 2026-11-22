package com.peopleplatform.performance.integration

import com.networknt.schema.InputFormat
import com.networknt.schema.JsonSchemaFactory
import com.networknt.schema.SpecVersion
import java.nio.file.Path
import org.yaml.snakeyaml.Yaml
import kotlin.test.fail

/** Valida payloads publicados contra os JSON Schemas de `contracts/asyncapi/schemas` (sem rede). */
object ContractSchemas {
    val contractsDir: Path = Path.of(System.getProperty("contracts.dir", "../../contracts")).toAbsolutePath().normalize()

    private val factory = JsonSchemaFactory.getInstance(SpecVersion.VersionFlag.V202012) { builder ->
        builder.schemaMappers {
            it.mapPrefix(
                "https://people-platform.example.com/schemas/",
                contractsDir.resolve("asyncapi/schemas").toUri().toString(),
            )
        }
    }

    fun assertValid(schemaFile: String, payload: String) {
        val schema = factory.getSchema(contractsDir.resolve("asyncapi/schemas/$schemaFile").toUri())
        val errors = schema.validate(payload, InputFormat.JSON)
        if (errors.isNotEmpty()) fail("payload fora do contrato $schemaFile: ${errors.joinToString { it.message }}\n$payload")
    }

    fun openApi(): Map<*, *> =
        contractsDir.resolve("openapi/performance-service.yaml").toFile().reader().use { Yaml().load<Map<*, *>>(it) }
}
