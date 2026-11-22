import { computed } from 'vue'
import { useEmployees } from '@/features/performance/queries'

/** Nome de uma pessoa do tenant a partir do cadastro do performance-service; sem cadastro, um rótulo neutro. */
export function useNames() {
  const employees = useEmployees()
  const byId = computed(() => new Map((employees.data.value ?? []).map((e) => [e.id, e.name])))
  const name = (id: string) => byId.value.get(id) ?? 'Pessoa sem cadastro'
  return { employees: computed(() => employees.data.value ?? []), name }
}
