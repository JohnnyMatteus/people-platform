package com.peopleplatform.performance.domain.okr

import com.peopleplatform.performance.domain.shared.require
import java.math.BigDecimal
import java.math.RoundingMode

/**
 * Nível do objetivo no desdobramento empresa → equipe → pessoa. Um objetivo só se desdobra de outro de nível
 * mais alto, então a cadeia nunca fecha em ciclo e o objetivo da empresa nunca tem pai.
 */
enum class ObjectiveLevel {
    COMPANY,
    TEAM,
    INDIVIDUAL,
    ;

    fun canCascadeFrom(parent: ObjectiveLevel): Boolean = parent.ordinal < ordinal
}

enum class ObjectiveStatus { ACTIVE, CLOSED }

/** Valida a regra de desdobramento na criação: o pai precisa ser de nível mais alto. */
fun requireCascade(child: ObjectiveLevel, parent: ObjectiveLevel?) {
    if (child == ObjectiveLevel.COMPANY) {
        require(parent == null) { "objetivo da empresa não tem objetivo pai" }
    } else if (parent != null) {
        require(child.canCascadeFrom(parent)) {
            "objetivo $child não pode se desdobrar de um objetivo $parent"
        }
    }
}

/** Resultado-chave: sai de [start] e vai até [target], que pode ser menor (reduzir um indicador). */
data class KeyResultSpec(val title: String, val start: BigDecimal, val target: BigDecimal) {
    init {
        require(title.trim().length >= 3) { "título do resultado-chave precisa de ao menos 3 caracteres" }
        require(start.compareTo(target) != 0) { "a meta do resultado-chave precisa ser diferente do ponto de partida" }
    }
}

object OkrProgress {
    private val ONE = BigDecimal.ONE.setScale(4)
    private val ZERO = BigDecimal.ZERO.setScale(4)

    /** Fração concluída de um resultado-chave, limitada a [0, 1]; funciona também para metas decrescentes. */
    fun keyResult(start: BigDecimal, target: BigDecimal, current: BigDecimal): BigDecimal {
        require(start.compareTo(target) != 0) { "a meta precisa ser diferente do ponto de partida" }
        val fraction = (current - start).divide(target - start, 4, RoundingMode.HALF_UP)
        return fraction.max(ZERO).min(ONE)
    }

    /** Média simples dos resultados-chave; objetivo sem resultado-chave tem progresso zero. */
    fun objective(keyResults: List<BigDecimal>): BigDecimal =
        if (keyResults.isEmpty()) {
            ZERO
        } else {
            val sum = keyResults.fold(BigDecimal.ZERO) { a, b -> a + b }
            sum.divide(BigDecimal(keyResults.size), 4, RoundingMode.HALF_UP)
        }
}
