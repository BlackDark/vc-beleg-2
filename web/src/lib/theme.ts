export type Theme = "system" | "light" | "dark";

const key = "mahlzeit.theme";

function readStored(): Theme {
  try {
    const v = localStorage.getItem(key);
    if (v === "light" || v === "dark" || v === "system") return v;
  } catch {
    // storage unavailable: use the default
  }
  return "dark";
}

/** Applies the theme to <html> and remembers it for the next visit. */
export function applyTheme(theme: Theme) {
  try {
    localStorage.setItem(key, theme);
  } catch {
    // ignore
  }
  const dark =
    theme === "dark" || (theme === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.classList.toggle("dark", dark);
  document.querySelector('meta[name="theme-color"]')?.setAttribute("content", dark ? "#14201f" : "#0f766e");
}

export function applyStoredTheme() {
  applyTheme(readStored());
  window
    .matchMedia("(prefers-color-scheme: dark)")
    .addEventListener("change", () => applyTheme(readStored()));
}

/** Tracks whether <html> currently carries the dark class. */
export function subscribeDark(onChange: () => void): () => void {
  const observer = new MutationObserver(onChange);
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
  return () => observer.disconnect();
}

export const isDark = () => document.documentElement.classList.contains("dark");
