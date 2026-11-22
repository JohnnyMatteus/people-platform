import { format, parseISO } from 'date-fns'
import { ptBR } from 'date-fns/locale'

export const dateTime = (iso: string) => format(parseISO(iso), "dd 'de' MMM 'às' HH:mm", { locale: ptBR })
export const dateOnly = (iso: string) => format(parseISO(iso), 'dd/MM/yyyy', { locale: ptBR })
export const dateShort = (iso: string) => format(parseISO(iso), 'dd MMM', { locale: ptBR })
export const decimal = (s: string | number, digits = 2) =>
  Number(s).toLocaleString('pt-BR', { minimumFractionDigits: digits, maximumFractionDigits: digits })
