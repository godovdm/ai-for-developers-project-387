import {
  Alert,
  Button,
  Center,
  Container,
  Loader,
  Paper,
  PasswordInput,
  Stack,
  Text,
  Title,
} from "@mantine/core";
import { useEffect, useState, type SyntheticEvent } from "react";
import { api, readErrorMessage } from "../api/client";
import { useOwnerSession } from "../hooks/useOwnerSession";
import { AppLink, navigate } from "../router";

export function OwnerLoginPage() {
  const { state, reload } = useOwnerSession();
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (state.status === "ready" && state.required && state.authenticated) {
      navigate("/owner");
    }
  }, [state]);

  const handleSubmit = async (event: SyntheticEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await api.createOwnerSession({ ownerLoginRequest: { password } });
      navigate("/owner");
    } catch (cause) {
      setError(await readErrorMessage(cause, "Не удалось выполнить вход"));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Container size="sm" py="xl">
      <Paper withBorder p="lg" radius="md">
        <Stack gap="md">
          <Title order={1} size="h2">
            Вход владельца
          </Title>

          {state.status === "loading" && (
            <Center py="md">
              <Loader aria-label="Проверка сессии" />
            </Center>
          )}

          {state.status === "error" && (
            <Alert color="red" title="Не удалось проверить сессию">
              <Button variant="default" onClick={reload}>
                Попробовать снова
              </Button>
            </Alert>
          )}

          {state.status === "ready" && !state.required && (
            <Stack gap="md">
              <Alert color="yellow" title="Демонстрационный режим">
                Пароль не настроен, защита владельческих операций выключена.
              </Alert>
              <Text c="dimmed">
                Чтобы включить защиту, задайте переменную окружения
                OWNER_PASSWORD и перезапустите сервер.
              </Text>
              <Button
                component="a"
                href="/owner"
                className="app-primary"
                w="fit-content"
              >
                Перейти в область владельца
              </Button>
            </Stack>
          )}

          {state.status === "ready" && state.required && (
            <form
              onSubmit={(event) => {
                void handleSubmit(event);
              }}
            >
              <Stack gap="md">
                <PasswordInput
                  label="Пароль"
                  value={password}
                  required
                  autoComplete="current-password"
                  onChange={(event) => {
                    setPassword(event.currentTarget.value);
                  }}
                />
                {error !== null && <Alert color="red">{error}</Alert>}
                <Button
                  className="app-primary"
                  type="submit"
                  loading={submitting}
                  w="fit-content"
                >
                  Войти
                </Button>
                <AppLink to="/" size="sm">
                  Вернуться на главную
                </AppLink>
              </Stack>
            </form>
          )}
        </Stack>
      </Paper>
    </Container>
  );
}
