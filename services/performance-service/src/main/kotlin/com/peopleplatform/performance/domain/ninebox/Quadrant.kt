package com.peopleplatform.performance.domain.ninebox

/**
 * Os nove quadrantes da matriz desempenho × potencial. O número segue a leitura usual da
 * matriz: 1 no canto inferior esquerdo (baixo/baixo) e 9 no superior direito (alto/alto).
 * Os rótulos são da plataforma; o tenant pode exibi-los com outros nomes no front-end.
 */
enum class Quadrant(val number: Int, val performance: Level, val potential: Level, val label: String) {
    Q1(1, Level.LOW, Level.LOW, "Risco de desempenho"),
    Q2(2, Level.MEDIUM, Level.LOW, "Mantenedor"),
    Q3(3, Level.HIGH, Level.LOW, "Especialista confiável"),
    Q4(4, Level.LOW, Level.MEDIUM, "Em ajuste"),
    Q5(5, Level.MEDIUM, Level.MEDIUM, "Núcleo sólido"),
    Q6(6, Level.HIGH, Level.MEDIUM, "Alto desempenho"),
    Q7(7, Level.LOW, Level.HIGH, "Potencial travado"),
    Q8(8, Level.MEDIUM, Level.HIGH, "Potencial em ascensão"),
    Q9(9, Level.HIGH, Level.HIGH, "Talento-chave"),
    ;

    companion object {
        fun of(performance: Level, potential: Level): Quadrant =
            entries.first { it.performance == performance && it.potential == potential }
    }
}
