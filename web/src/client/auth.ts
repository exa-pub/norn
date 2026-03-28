const COOKIE_NAME = "norn_secret";

export function initAuth(): boolean {
  // Read secret from URL fragment.
  const hash = window.location.hash;
  const match = hash.match(/nornSecret=([a-f0-9]+)/);
  if (match) {
    document.cookie = `${COOKIE_NAME}=${match[1]}; path=/; SameSite=Strict`;
    // Remove fragment from URL.
    history.replaceState(null, "", window.location.pathname + window.location.search);
  }

  // Check if cookie exists.
  return document.cookie.split(";").some((c) => c.trim().startsWith(`${COOKIE_NAME}=`));
}
