#!/usr/bin/env bash
# Transforma o log de um passo que falhou em até 5 anotações ::error do GitHub Actions, para o erro
# aparecer na página do check sem login. Nunca lê o ambiente: só o arquivo de log passado.
# Uso: ci-annotate.sh <título> <arquivo-de-log> [regex-das-linhas-relevantes] [linhas-de-cauda]
set -uo pipefail

title="${1:?título}"
log="${2:?arquivo de log}"
pattern="${3:-}"
tail_n="${4:-40}"

if [ ! -f "$log" ]; then
  echo "::error title=${title}::sem log em ${log} (o passo parou antes de gravar)"
  exit 0
fi

clean="$(mktemp)"
picked="$(mktemp)"
# Tira cores ANSI e CR, e encurta o caminho absoluto do runner.
sed -e 's/\x1b\[[0-9;]*[A-Za-z]//g' -e 's/\r$//' -e 's#/home/runner/work/[^/]*/[^/]*/##g' "$log" > "$clean"
if [ -n "$pattern" ] && grep -Eq "$pattern" "$clean"; then
  grep -E "$pattern" "$clean" | head -n 300 > "$picked"
else
  tail -n "$tail_n" "$clean" > "$picked"
fi

awk -v title="$title" -v max=3500 -v limit=5 '
  function flush() {
    if (buf != "" && n < limit) { print "::error title=" title "::" buf; n++ }
    buf = ""
  }
  {
    line = substr($0, 1, 400)
    gsub(/%/, "%25", line)
    gsub(/\r/, "%0D", line)
    if (length(buf) + length(line) + 3 > max) flush()
    buf = (buf == "") ? line : buf "%0A" line
  }
  END { flush() }
' "$picked"
exit 0
