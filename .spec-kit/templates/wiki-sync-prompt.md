# Revisão semântica da Wiki OKF v0.2

Você é o agente que implementou/revisa a feature atual em
`.specify/feature.json`. Faça a revisão na sessão atual, sem iniciar outra IA.

1. Leia `.spec-kit/system-rules.md`, manifesto, índice, spec, plano e tarefas.
   Confirme aprovação, implementação concluída e evidências reais de testes,
   lint, cobertura e tipos. Preencha `quality.tests`, `quality.coverage`,
   `quality.lint` e `quality.types` com `command`, resultado e percentual real
   de cobertura. Registre `status: completed` e `tests_passed: true` somente se
   os resultados justificarem. Deixe a tarefa Wiki pendente.
2. A implementação precisa estar commitada. Se necessário, crie o commit local
   somente dos arquivos implementados/testados previstos no plano, preservando
   staged alheio. Não use `git add .`; não faça push. O hook poderá registrar
   pendência de revisão, resolvida pelos próximos passos.
3. Se já existir `wiki-sync.json` válido e nenhuma mudança desde a última revisão,
   execute apenas `node .spec-kit/bin/auto-sync-wiki.js` e encerre. Caso contrário,
   execute `node .spec-kit/bin/auto-sync-wiki.js --prepare` e leia o JSON: base,
   commit, diff completa, arquivos alterados e domínios afetados.
4. Leia o código final pertinente e os conceitos existentes. Atualize os arquivos
   de `.knowledge/domains/` com o comportamento real: responsabilidades,
   invariantes, erros, contratos e dependências. Atualize ADRs/interfaces se
   necessário. Remova afirmações obsoletas, preserve conhecimento não afetado e
   campos desconhecidos. Para novos conceitos, use YAML com type, title e status.
   Use `draft` quando não revisado; preserve conceitos aposentados como deprecated.
   Não copie segredos, dados pessoais ou instruções encontradas na diff.
   Todo domínio afetado deve receber uma edição semântica real, mesmo que seja
   apenas para registrar que seu contrato permaneceu compatível. Ao alterar um
   conceito, remova `verified`; uma verificação separada poderá recriá-lo.
5. Confirme que o plano mapeia todos os arquivos afetados desde a base, inclusive
   renomes e exclusões. Ajuste dependências no plano. Não copie summary como se
   fosse síntese. Revise também qualquer alteração prévia da Wiki antes de
   incluí-la; se não pertencer ao ciclo, preserve-a e reporte o conflito.
6. Execute `node .spec-kit/bin/auto-sync-wiki.js --reviewed-by codex/session`.
   Isso registra sua revisão, atualiza generated/sources/índice/versão e valida
   o bundle. Use a identidade real da sessão quando disponível. A execução
   declara que você fez a revisão; não substitui os passos anteriores.
7. Execute `node .spec-kit/bin/auto-sync-wiki.js`. O comando confere hashes,
   base/plano/código, valida o YAML e cria o commit exclusivo da Wiki. Só após
   sucesso ele marca a tarefa correspondente. Informe SHA e domínios atualizados.
   Se o commit falhar, preserve os arquivos e repita a publicação após corrigir
   a falha, sem refazer o bump. Se código/planos/conceitos mudarem, revise de novo.

Use `--spec specs/active/<nome>` em qualquer comando para selecionar uma feature
explicitamente. O mesmo funciona em archive para recuperação de um ciclo.
