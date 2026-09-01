import { StrictMode, useState } from "react";
import { createRoot } from "react-dom/client";
import { App, defaultBrowserRepository } from "./App";
import {
  beginOIDCSignIn, clearAccessToken, completeOIDCCallback, currentAccessToken, oidcConfigured, storeLocalToken,
} from "./runtime/oidc";
import { ServerFarmRepository } from "./runtime/serverRepository";
import "./index.css";

const runtimeMode = (import.meta.env.VITE_RUNTIME_MODE === "server" ? "server" : "browser") as "server" | "browser";

function ServerSignIn({ callbackError }: { callbackError?: string }) {
  const [token, setToken] = useState("");
  const [error, setError] = useState(callbackError);
  const oidc = oidcConfigured();

  const signInOIDC = async () => {
    try {
      setError(undefined);
      await beginOIDCSignIn();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Sign in failed");
    }
  };
  const signInLocal = () => {
    try {
      setError(undefined);
      storeLocalToken(token);
      location.reload();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Sign in failed");
    }
  };

  return <main className="gn-welcome"><section><p className="gn-eyebrow">Authenticated server runtime</p><h1>Sign in to GrowNerve</h1><p>{oidc ? "Use the configured identity provider. GrowNerve uses Authorization Code with PKCE and keeps the access token only for this browser session." : "Enter a local bearer token configured by the GrowNerve server administrator."}</p>{error && <p className="gn-error">{error}</p>}{oidc ? <button className="gn-button primary" onClick={() => void signInOIDC()}>Sign in with identity provider</button> : <form className="gn-form" onSubmit={(event) => { event.preventDefault(); signInLocal(); }}><label>Bearer token<input type="password" autoComplete="current-password" value={token} onChange={(event) => setToken(event.target.value)} /></label><button className="gn-button primary" type="submit">Sign in</button></form>}</section></main>;
}

async function bootstrap() {
  const root = createRoot(document.getElementById("root")!);
  if (runtimeMode === "browser") {
    root.render(<StrictMode><App repository={defaultBrowserRepository} runtimeMode="browser" /></StrictMode>);
    return;
  }

  let callbackError: string | undefined;
  if (oidcConfigured()) {
    try {
      await completeOIDCCallback();
    } catch (cause) {
      callbackError = cause instanceof Error ? cause.message : "Identity-provider callback failed";
    }
  }
  if (!currentAccessToken()) {
    root.render(<StrictMode><ServerSignIn callbackError={callbackError} /></StrictMode>);
    return;
  }

  const repository = new ServerFarmRepository(import.meta.env.VITE_API_URL || "http://127.0.0.1:8080", {
    token: currentAccessToken,
    onUnauthorized: () => {
      clearAccessToken();
      location.reload();
    },
  });
  root.render(<StrictMode><App repository={repository} runtimeMode="server" /></StrictMode>);
}

void bootstrap();
