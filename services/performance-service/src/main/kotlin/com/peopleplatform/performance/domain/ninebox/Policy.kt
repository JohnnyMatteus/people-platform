package com.peopleplatform.performance.domain.ninebox

import com.peopleplatform.performance.domain.shared.TenantId
import com.peopleplatform.performance.domain.shared.require
import java.math.BigDecimal

/** Faixa de um eixo da matriz. */
enum class Level { LOW, MEDIUM, HIGH }

/**
 * Limiares de um eixo numa escala contínua [scaleMin, scaleMax].
 *
 * - nota < [mediumFrom] é LOW;
 * - [mediumFrom] <= nota < [highFrom] é MEDIUM;
 * - nota >= [highFrom] é HIGH.
 *
 * Os limites inferiores são inclusivos para que uma nota exatamente no limiar suba de faixa.
 * BigDecimal evita que 3.75 vire 3.7499999 e mude a faixa de alguém por erro de ponto flutuante.
 */
data class AxisThresholds(val mediumFrom: BigDecimal, val highFrom: BigDecimal) {
    fun levelOf(score: BigDecimal): Level = when {
        score < mediumFrom -> Level.LOW
        score < highFrom -> Level.MEDIUM
        else -> Level.HIGH
    }
}

/** Escala de notas usada pelo tenant (ex.: 1 a 5, 0 a 10). */
data class Scale(val min: BigDecimal, val max: BigDecimal) {
    init {
        require(min < max) { "escala inválida: mínimo $min deve ser menor que máximo $max" }
    }

    operator fun contains(score: BigDecimal): Boolean = score in min..max
}

/**
 * Política de nine box de um tenant: escala, limiares por eixo e regras de calibração.
 *
 * @property maxStepsPerAxis quantas faixas a calibração pode mover em cada eixo. O padrão (1)
 *   impede que um comitê leve alguém de LOW para HIGH sem reavaliação formal.
 * @property topBoxWarningShare fatia máxima recomendada no quadrante de topo; acima dela a
 *   sessão de calibração emite um alerta (não bloqueia: distribuição forçada é decisão do RH).
 * @property minJustificationLength tamanho mínimo da justificativa de um ajuste.
 */
data class NineBoxPolicy(
    val tenantId: TenantId,
    val scale: Scale,
    val performance: AxisThresholds,
    val potential: AxisThresholds,
    val maxStepsPerAxis: Int = 1,
    val topBoxWarningShare: BigDecimal = BigDecimal("0.20"),
    val minJustificationLength: Int = 20,
) {
    init {
        listOf("desempenho" to performance, "potencial" to potential).forEach { (axis, t) ->
            require(scale.min < t.mediumFrom && t.mediumFrom < t.highFrom && t.highFrom <= scale.max) {
                "limiares de $axis devem satisfazer " +
                    "${scale.min} < médio(${t.mediumFrom}) < alto(${t.highFrom}) <= ${scale.max}"
            }
        }
        require(maxStepsPerAxis in 1..2) { "maxStepsPerAxis deve ser 1 ou 2" }
        require(topBoxWarningShare > BigDecimal.ZERO && topBoxWarningShare <= BigDecimal.ONE) {
            "topBoxWarningShare deve estar em (0, 1]"
        }
        require(minJustificationLength >= 0) { "minJustificationLength não pode ser negativo" }
    }

    companion object {
        /** Padrão da plataforma: escala 1–5 com terços aproximados (2.5 e 3.75). */
        fun default(tenantId: TenantId) = NineBoxPolicy(
            tenantId = tenantId,
            scale = Scale(BigDecimal("1"), BigDecimal("5")),
            performance = AxisThresholds(BigDecimal("2.5"), BigDecimal("3.75")),
            potential = AxisThresholds(BigDecimal("2.5"), BigDecimal("3.75")),
        )
    }
}
