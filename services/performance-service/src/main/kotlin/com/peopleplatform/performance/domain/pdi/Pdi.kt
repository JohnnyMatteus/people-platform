package com.peopleplatform.performance.domain.pdi

import com.peopleplatform.performance.domain.evaluation.CompetencyGap
import com.peopleplatform.performance.domain.ninebox.Level
import com.peopleplatform.performance.domain.ninebox.Quadrant
import com.peopleplatform.performance.domain.shared.require

enum class PdiActionKind { COURSE, MENTORING, PROJECT, READING }

/** PLANNED → IN_PROGRESS → DONE. Uma ação concluída é registro: não reabre. */
enum class PdiActionStatus {
    PLANNED,
    IN_PROGRESS,
    DONE,
    ;

    fun moveTo(target: PdiActionStatus): PdiActionStatus {
        require(target.ordinal >= ordinal && !(this == DONE && target != DONE)) {
            "ação não pode voltar de $this para $target"
        }
        return target
    }
}

data class PdiSuggestion(val title: String, val competency: String?, val kind: PdiActionKind, val reason: String)

/**
 * Sugestões de PDI a partir dos gaps da avaliação e do quadrante do nine box. São só sugestões:
 * nada vira ação sem confirmação de uma pessoa.
 */
object PdiAdvisor {
    /** Gaps de 2 pontos ou mais viram sugestão; 1 ponto é ruído de calibração entre avaliadores. */
    const val MIN_GAP = 2

    fun suggest(gaps: List<CompetencyGap>, quadrant: Quadrant?): List<PdiSuggestion> {
        val fromGaps = gaps.filter { it.gap >= MIN_GAP }.sortedByDescending { it.gap }.map {
            PdiSuggestion(
                title = "Conversa de alinhamento sobre ${it.competency}",
                competency = it.competency,
                kind = PdiActionKind.MENTORING,
                reason = "Autoavaliação e gestor divergem em ${it.gap} pontos nesta competência.",
            )
        }
        return fromGaps + listOfNotNull(quadrant?.let(::fromQuadrant))
    }

    private fun fromQuadrant(q: Quadrant): PdiSuggestion? = when {
        q.performance == Level.LOW -> PdiSuggestion(
            "Plano de recuperação de entregas com o gestor", null, PdiActionKind.PROJECT,
            "Nine box em ${q.name} (${q.label}): desempenho abaixo do esperado.",
        )
        q.potential == Level.HIGH -> PdiSuggestion(
            "Projeto de escopo ampliado ou mentoria de liderança", null, PdiActionKind.PROJECT,
            "Nine box em ${q.name} (${q.label}): potencial alto pede desafio maior.",
        )
        else -> null
    }
}
