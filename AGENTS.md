## CRITICAL RULES — ОБЯЗАТЕЛЬНО

- NEVER удалять или переписывать рабочие тесты без явного запроса
- NEVER удалять файлы без подтверждения
- ALWAYS запускать тесты после любого изменения кода
- ALWAYS делать git checkpoint перед крупными рефакторингами
- Одна задача за раз. НЕ делать несколько изменений одновременно
- Если не уверен — СПРОСИ, не угадывай

## Working Style

- ДЛЯ ЗАДАЧ СОЗДАНИЯ merge request ИСПОЛЬЗУЙ merge-requester без планировщика
- ЕСЛИ ЗАДАЧА НА РАЗРАБОТКУ: Сначала ПЛАН, потом код
- Маленькие дифы: один файл → тесты → следующий файл
- Используй субагентов для исследования кодовой базы
- Сообщения коммитов оформляй по Conventional Commits: `type(scope): описание` (scope необязателен).
- Перед завершением изменений запускай `mise --locked run check` и добивайся успешного результата.
- Если на macOS Firefox завершается с `Could not find profile folder`, создай `/private/tmp/calendar-firefox-home` и запусти проверку с `JAVA_HOME="$(/usr/libexec/java_home)" CFFIXED_USER_HOME=/private/tmp/calendar-firefox-home mise --locked run check`.
- Не редактируй вручную сгенерированные `mise.lock` и `.mise/locks/`.
- Не отключай правила проверок ради прохождения: узкое исключение с причиной требует решения человека.

## Agents

- Use `planner` agent для планирования
- Use `tester` agent после изменений кода
- Use `code-reviewer` agent перед коммитами

## Agent skills

### Issue tracker

Track issues and specs in GitHub Issues for
`godovdm/ai-for-developers-project-387`.
Read `docs/agents/issue-tracker.md` before tracker operations.

### Triage labels

Use the five default triage labels.
Read `docs/agents/triage-labels.md` when assigning triage roles.

### Domain docs

Use a single-context layout: root `GLOSSARY.md` and `docs/adr/`.
Read `docs/agents/domain.md` before exploring domain concepts or decisions.

### GitHub agent

The agent in GitHub Actions runs the same skills and instructions as the local
one: the workflow is `.github/workflows/opencode.yml`, and project skills live in
`.agents/skills`. Keep the commands in this file runnable on a clean runner —
the workflow installs project tools with mise before the agent starts.
