const TOKEN_KEY = "grownerve.token";
const TOKEN_EXPIRY_KEY = "grownerve.token_expiry";
const VERIFIER_KEY = "grownerve.oidc.verifier";
const STATE_KEY = "grownerve.oidc.state";

interface OIDCMetadata {
  issuer: string;
  authorization_endpoint: string;
  token_endpoint: string;
}

interface TokenResponse {
  access_token: string;
  expires_in?: number;
  token_type?: string;
}

function configuredIssuer(): string | undefined {
  const value = import.meta.env.VITE_OIDC_ISSUER?.trim();
  return value ? value.replace(/\/$/, "") : undefined;
}

function configuredClientID(): string | undefined {
  const value = import.meta.env.VITE_OIDC_CLIENT_ID?.trim();
  return value || undefined;
}

export function oidcConfigured(): boolean {
  return Boolean(configuredIssuer() && configuredClientID());
}

export function currentAccessToken(): string | undefined {
  try {
    const token = sessionStorage.getItem(TOKEN_KEY) ?? undefined;
    const expiry = Number(sessionStorage.getItem(TOKEN_EXPIRY_KEY) ?? "0");
    if (token && expiry > Date.now() + 30_000) return token;
    if (expiry > 0) clearAccessToken();
    return token && expiry === 0 ? token : undefined;
  } catch {
    return undefined;
  }
}

export function storeLocalToken(token: string): void {
  const trimmed = token.trim();
  if (!trimmed) throw new Error("A bearer token is required.");
  sessionStorage.setItem(TOKEN_KEY, trimmed);
  sessionStorage.removeItem(TOKEN_EXPIRY_KEY);
}

export function clearAccessToken(): void {
  try {
    sessionStorage.removeItem(TOKEN_KEY);
    sessionStorage.removeItem(TOKEN_EXPIRY_KEY);
    sessionStorage.removeItem(VERIFIER_KEY);
    sessionStorage.removeItem(STATE_KEY);
  } catch {
    // Browsers with storage disabled simply return to the sign-in gate.
  }
}

function callbackURL(): string {
  return `${location.origin}${location.pathname}`;
}

function randomURLSafe(bytes: number): string {
  const value = new Uint8Array(bytes);
  crypto.getRandomValues(value);
  return base64URL(value);
}

function base64URL(value: Uint8Array): string {
  let binary = "";
  value.forEach((byte) => { binary += String.fromCharCode(byte); });
  return btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}

async function sha256(value: string): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(value));
  return base64URL(new Uint8Array(digest));
}

async function metadata(): Promise<OIDCMetadata> {
  const issuer = configuredIssuer();
  if (!issuer) throw new Error("OIDC issuer is not configured.");
  const response = await fetch(`${issuer}/.well-known/openid-configuration`, { headers: { Accept: "application/json" } });
  if (!response.ok) throw new Error(`Identity provider metadata failed: ${response.status}`);
  const document = await response.json() as Partial<OIDCMetadata>;
  if (document.issuer?.replace(/\/$/, "") !== issuer || !document.authorization_endpoint || !document.token_endpoint) {
    throw new Error("Identity provider metadata does not match the configured issuer.");
  }
  return document as OIDCMetadata;
}

export async function beginOIDCSignIn(): Promise<void> {
  const clientID = configuredClientID();
  if (!clientID) throw new Error("OIDC client ID is not configured.");
  const provider = await metadata();
  const verifier = randomURLSafe(64);
  const state = randomURLSafe(32);
  sessionStorage.setItem(VERIFIER_KEY, verifier);
  sessionStorage.setItem(STATE_KEY, state);

  const parameters = new URLSearchParams({
    response_type: "code",
    client_id: clientID,
    redirect_uri: callbackURL(),
    scope: import.meta.env.VITE_OIDC_SCOPES?.trim() || "openid profile email",
    code_challenge: await sha256(verifier),
    code_challenge_method: "S256",
    state,
  });
  location.assign(`${provider.authorization_endpoint}?${parameters}`);
}

export async function completeOIDCCallback(): Promise<boolean> {
  if (!oidcConfigured()) return false;
  const query = new URLSearchParams(location.search);
  const code = query.get("code");
  const returnedState = query.get("state");
  const providerError = query.get("error");
  if (providerError) {
    throw new Error(query.get("error_description") || providerError);
  }
  if (!code && !returnedState) return false;
  if (!code || !returnedState) throw new Error("Incomplete identity-provider callback.");

  const verifier = sessionStorage.getItem(VERIFIER_KEY);
  const expectedState = sessionStorage.getItem(STATE_KEY);
  if (!verifier || !expectedState || returnedState !== expectedState) {
    clearAccessToken();
    throw new Error("Identity-provider callback state did not match the login request.");
  }

  const clientID = configuredClientID();
  if (!clientID) throw new Error("OIDC client ID is not configured.");
  const provider = await metadata();
  const body = new URLSearchParams({
    grant_type: "authorization_code",
    client_id: clientID,
    redirect_uri: callbackURL(),
    code,
    code_verifier: verifier,
  });
  const response = await fetch(provider.token_endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded", Accept: "application/json" },
    body,
  });
  if (!response.ok) throw new Error(`Identity-provider token exchange failed: ${response.status}`);
  const token = await response.json() as Partial<TokenResponse>;
  if (!token.access_token) throw new Error("Identity provider returned no access token.");

  sessionStorage.setItem(TOKEN_KEY, token.access_token);
  const lifetime = Math.max(30, token.expires_in ?? 300);
  sessionStorage.setItem(TOKEN_EXPIRY_KEY, String(Date.now() + lifetime * 1000));
  sessionStorage.removeItem(VERIFIER_KEY);
  sessionStorage.removeItem(STATE_KEY);
  history.replaceState(null, "", `${location.pathname}${location.hash}`);
  return true;
}
