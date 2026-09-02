const TOKEN_KEY = "grownerve.token";
const TOKEN_EXPIRY_KEY = "grownerve.token_expiry";
const VERIFIER_KEY = "grownerve.oidc.verifier";
const STATE_KEY = "grownerve.oidc.state";
const DEFAULT_TOKEN_LIFETIME_SECONDS = 300;
const MINIMUM_TOKEN_LIFETIME_SECONDS = 30;
const LOCAL_HOSTS = new Set(["localhost", "127.0.0.1", "[::1]"]);

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
    if (token && Number.isFinite(expiry) && expiry > Date.now() + 30_000) return token;
    if (expiry !== 0) clearAccessToken();
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

function clearOIDCTransaction(): void {
  try {
    sessionStorage.removeItem(VERIFIER_KEY);
    sessionStorage.removeItem(STATE_KEY);
  } catch {
    // Storage-disabled browsers cannot complete a PKCE transaction anyway.
  }
}

export function clearAccessToken(): void {
  try {
    sessionStorage.removeItem(TOKEN_KEY);
    sessionStorage.removeItem(TOKEN_EXPIRY_KEY);
  } catch {
    // Browsers with storage disabled simply return to the sign-in gate.
  }
  clearOIDCTransaction();
}

function callbackURL(): string {
  return `${location.origin}${location.pathname}`;
}

function secureURL(value: string, label: string): URL {
  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    throw new Error(`${label} is not a valid URL.`);
  }
  if (parsed.protocol === "https:") return parsed;
  if (parsed.protocol === "http:" && LOCAL_HOSTS.has(parsed.hostname)) return parsed;
  throw new Error(`${label} must use HTTPS outside localhost.`);
}

function cleanCallbackParameters(): void {
  const url = new URL(location.href);
  for (const key of ["code", "state", "error", "error_description", "error_uri", "session_state", "iss"]) {
    url.searchParams.delete(key);
  }
  history.replaceState(null, "", `${url.pathname}${url.search}${url.hash}`);
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
  secureURL(issuer, "OIDC issuer");
  const response = await fetch(`${issuer}/.well-known/openid-configuration`, { headers: { Accept: "application/json" } });
  if (!response.ok) throw new Error(`Identity provider metadata failed: ${response.status}`);
  const document = await response.json() as Partial<OIDCMetadata>;
  if (document.issuer?.replace(/\/$/, "") !== issuer || !document.authorization_endpoint || !document.token_endpoint) {
    throw new Error("Identity provider metadata does not match the configured issuer.");
  }
  secureURL(document.authorization_endpoint, "OIDC authorization endpoint");
  secureURL(document.token_endpoint, "OIDC token endpoint");
  return document as OIDCMetadata;
}

export async function beginOIDCSignIn(): Promise<void> {
  const clientID = configuredClientID();
  if (!clientID) throw new Error("OIDC client ID is not configured.");
  secureURL(callbackURL(), "GrowNerve callback URL");
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
    clearOIDCTransaction();
    cleanCallbackParameters();
    throw new Error(query.get("error_description") || providerError);
  }
  if (!code && !returnedState) return false;
  if (!code || !returnedState) {
    clearOIDCTransaction();
    cleanCallbackParameters();
    throw new Error("Incomplete identity-provider callback.");
  }

  const verifier = sessionStorage.getItem(VERIFIER_KEY);
  const expectedState = sessionStorage.getItem(STATE_KEY);
  if (!verifier || !expectedState || returnedState !== expectedState) {
    clearAccessToken();
    cleanCallbackParameters();
    throw new Error("Identity-provider callback state did not match the login request.");
  }

  const clientID = configuredClientID();
  if (!clientID) throw new Error("OIDC client ID is not configured.");
  secureURL(callbackURL(), "GrowNerve callback URL");

  // The authorization response is single-use. Remove the code and PKCE
  // transaction before exchanging it so a failed exchange cannot be replayed by
  // refreshing the page or copied from browser history.
  clearOIDCTransaction();
  cleanCallbackParameters();

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
  if (token.token_type && token.token_type.toLowerCase() !== "bearer") {
    throw new Error("Identity provider returned an unsupported token type.");
  }

  const reportedLifetime = token.expires_in ?? DEFAULT_TOKEN_LIFETIME_SECONDS;
  if (!Number.isFinite(reportedLifetime) || reportedLifetime <= 0) {
    throw new Error("Identity provider returned an invalid token lifetime.");
  }
  const lifetime = Math.max(MINIMUM_TOKEN_LIFETIME_SECONDS, reportedLifetime);
  sessionStorage.setItem(TOKEN_KEY, token.access_token);
  sessionStorage.setItem(TOKEN_EXPIRY_KEY, String(Date.now() + lifetime * 1000));
  return true;
}
