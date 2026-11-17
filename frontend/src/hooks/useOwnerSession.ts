import { useCallback, useEffect, useState } from "react";
import { api } from "../api/client";

export type OwnerSessionState =
  | { status: "loading" }
  | { status: "ready"; required: boolean; authenticated: boolean }
  | { status: "error" };

export function useOwnerSession() {
  const [state, setState] = useState<OwnerSessionState>({
    status: "loading",
  });
  const [request, setRequest] = useState(0);

  useEffect(() => {
    let active = true;
    void api.getOwnerSession().then(
      (session) => {
        if (active) {
          setState({
            status: "ready",
            required: session.required,
            authenticated: session.authenticated,
          });
        }
      },
      () => {
        if (active) setState({ status: "error" });
      },
    );
    return () => {
      active = false;
    };
  }, [request]);

  const reload = useCallback(() => {
    setState({ status: "loading" });
    setRequest((value) => value + 1);
  }, []);

  return { state, reload };
}
