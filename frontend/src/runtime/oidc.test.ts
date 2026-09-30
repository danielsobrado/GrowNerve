import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { beginOIDCSignIn, clearAccessToken, completeOIDCCallback, currentAccessToken, oidcConfigured, storeLocalToken } from "./oidc";

const ISSUER = "https://id.example.com/realms/farm";
const APP = "https://farm.example.com/app/";

const discovery = (overrides: Record<string, unknown> = {}) => new Response(JSON.stringify({
  issuer: ISSUER, authorization_endpoint: `${ISSUER}/auth`, token_endpoint: `${ISSUER}/token`, ...overrides,
}), { status: 200, headers: { "Content-Type": "application/json" } });
const token = (body: Record<string, unknown>, status = 200) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

/** jsdom's location cannot be spied on, so each test runs against a stand-in at the app's callback URL. */
function atURL(href: string) {
  const url = new URL(href);
  const fake = { href: url.href, origin: url.origin, pathname: url.pathname, search: url.search, hash: url.hash, assign: vi.fn() };
  vi.stubGlobal("location", fake);
  return fake;
}

async function challengeFor(verifier: string) {
  const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier)));
  return btoa(String.fromCharCode(...digest)).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}

beforeEach(() => {
  sessionStorage.clear();
  vi.stubEnv("VITE_OIDC_ISSUER", `${ISSUER}/`);
  vi.stubEnv("VITE_OIDC_CLIENT_ID", "grownerve-web");
  vi.spyOn(history, "replaceState").mockImplementation(() => undefined);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
});

describe("bearer token storage", () => {
  it("keeps a local token without expiry and refuses a blank one", () => {
    expect(() => storeLocalToken("   ")).toThrow("A bearer token is required.");
    storeLocalToken("  local-secret ");
    expect(currentAccessToken()).toBe("local-secret");
    clearAccessToken();
    expect(currentAccessToken()).toBeUndefined();
  });

  it("drops an identity-provider token that is about to expire", () => {
    sessionStorage.setItem("grownerve.token", "fresh");
    sessionStorage.setItem("grownerve.token_expiry", String(Date.now() + 120_000));
    expect(currentAccessToken()).toBe("fresh");
    sessionStorage.setItem("grownerve.token_expiry", String(Date.now() + 10_000));
    expect(currentAccessToken()).toBeUndefined();
    expect(sessionStorage.getItem("grownerve.token")).toBeNull();
  });

  it("reports OIDC as configured only with both issuer and client", () => {
    expect(oidcConfigured()).toBe(true);
    vi.stubEnv("VITE_OIDC_CLIENT_ID", "");
    expect(oidcConfigured()).toBe(false);
  });
});

describe("OIDC authorization code flow with PKCE", () => {
  it("redirects to the provider with an S256 challenge for a stored verifier", async () => {
    const location = atURL(APP);
    const fetch = vi.fn().mockResolvedValueOnce(discovery());
    vi.stubGlobal("fetch", fetch);
    await beginOIDCSignIn();

    expect(fetch).toHaveBeenCalledWith(`${ISSUER}/.well-known/openid-configuration`, expect.anything());
    const redirect = new URL(location.assign.mock.calls[0][0] as string);
    const verifier = sessionStorage.getItem("grownerve.oidc.verifier")!;
    expect(`${redirect.origin}${redirect.pathname}`).toBe(`${ISSUER}/auth`);
    expect(Object.fromEntries(redirect.searchParams)).toMatchObject({
      response_type: "code", client_id: "grownerve-web", redirect_uri: APP, scope: "openid profile email",
      code_challenge_method: "S256", state: sessionStorage.getItem("grownerve.oidc.state"),
    });
    expect(redirect.searchParams.get("code_challenge")).toBe(await challengeFor(verifier));
  });

  it("refuses metadata for another issuer and insecure endpoints", async () => {
    atURL(APP);
    vi.stubGlobal("fetch", vi.fn()
      .mockResolvedValueOnce(discovery({ issuer: "https://evil.example.com" }))
      .mockResolvedValueOnce(discovery({ token_endpoint: "http://id.example.com/token" }))
      .mockResolvedValueOnce(new Response(null, { status: 503 })));
    await expect(beginOIDCSignIn()).rejects.toThrow("does not match the configured issuer");
    await expect(beginOIDCSignIn()).rejects.toThrow("must use HTTPS outside localhost");
    await expect(beginOIDCSignIn()).rejects.toThrow("metadata failed: 503");
    vi.stubEnv("VITE_OIDC_ISSUER", "http://id.example.com");
    await expect(beginOIDCSignIn()).rejects.toThrow("must use HTTPS outside localhost");
    vi.stubEnv("VITE_OIDC_ISSUER", "not a url");
    await expect(beginOIDCSignIn()).rejects.toThrow("is not a valid URL");
    atURL("http://farm.example.com/app/");
    await expect(beginOIDCSignIn()).rejects.toThrow("callback URL must use HTTPS");
  });

  it("exchanges the code once, clears the transaction first and stores the token with its lifetime", async () => {
    sessionStorage.setItem("grownerve.oidc.verifier", "verifier-1");
    sessionStorage.setItem("grownerve.oidc.state", "state-1");
    atURL(`${APP}?code=code-1&state=state-1&session_state=s`);
    const fetch = vi.fn().mockResolvedValueOnce(discovery()).mockResolvedValueOnce(token({ access_token: "access-1", token_type: "Bearer", expires_in: 600 }));
    vi.stubGlobal("fetch", fetch);

    expect(await completeOIDCCallback()).toBe(true);
    const [url, init] = fetch.mock.calls[1];
    expect(url).toBe(`${ISSUER}/token`);
    expect(Object.fromEntries(init.body as URLSearchParams)).toEqual({
      grant_type: "authorization_code", client_id: "grownerve-web", redirect_uri: APP, code: "code-1", code_verifier: "verifier-1",
    });
    expect(history.replaceState).toHaveBeenCalledWith(null, "", "/app/");
    expect(sessionStorage.getItem("grownerve.oidc.verifier")).toBeNull();
    expect(currentAccessToken()).toBe("access-1");
    expect(Number(sessionStorage.getItem("grownerve.token_expiry"))).toBeGreaterThan(Date.now() + 590_000);
  });

  it("does nothing without a callback or configuration", async () => {
    atURL(APP);
    expect(await completeOIDCCallback()).toBe(false);
    vi.stubEnv("VITE_OIDC_ISSUER", "");
    atURL(`${APP}?code=c&state=s`);
    expect(await completeOIDCCallback()).toBe(false);
  });

  it("rejects provider errors, incomplete callbacks and a state that does not match", async () => {
    atURL(`${APP}?error=access_denied&error_description=User%20cancelled`);
    await expect(completeOIDCCallback()).rejects.toThrow("User cancelled");
    atURL(`${APP}?code=only-code`);
    await expect(completeOIDCCallback()).rejects.toThrow("Incomplete identity-provider callback.");
    sessionStorage.setItem("grownerve.oidc.verifier", "verifier-1");
    sessionStorage.setItem("grownerve.oidc.state", "state-1");
    atURL(`${APP}?code=c&state=forged`);
    await expect(completeOIDCCallback()).rejects.toThrow("state did not match");
    expect(sessionStorage.getItem("grownerve.oidc.verifier")).toBeNull();
  });

  it.each([
    [token({}, 400), "token exchange failed: 400"],
    [token({ token_type: "Bearer" }), "returned no access token"],
    [token({ access_token: "a", token_type: "MAC" }), "unsupported token type"],
    [token({ access_token: "a", expires_in: -1 }), "invalid token lifetime"],
  ])("refuses an unusable token response (%#)", async (response, expected) => {
    sessionStorage.setItem("grownerve.oidc.verifier", "verifier-1");
    sessionStorage.setItem("grownerve.oidc.state", "state-1");
    atURL(`${APP}?code=c&state=state-1`);
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(discovery()).mockResolvedValueOnce(response));
    await expect(completeOIDCCallback()).rejects.toThrow(expected);
    expect(currentAccessToken()).toBeUndefined();
  });

  it("keeps a short-lived token for at least the minimum lifetime", async () => {
    sessionStorage.setItem("grownerve.oidc.verifier", "verifier-1");
    sessionStorage.setItem("grownerve.oidc.state", "state-1");
    atURL(`${APP}?code=c&state=state-1`);
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(discovery()).mockResolvedValueOnce(token({ access_token: "brief", expires_in: 5 })));
    await completeOIDCCallback();
    expect(Number(sessionStorage.getItem("grownerve.token_expiry"))).toBeGreaterThanOrEqual(Date.now() + 29_000);
  });
});
